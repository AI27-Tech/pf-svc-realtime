// Package connection es la gorutina por conexión SSE: un select sobre cierre
// del cliente, desalojo (revocación), latido y mensaje (realtime.md (c),
// ADR 28). Sobre net/http estándar, sin framework.
//
// Con el bus caído el latido sigue y el cliente no reconecta solo: por eso, al
// perder evidencia de frescura, el servidor emite el control tipado stream.stale
// y el cliente pasa a recuperación HTTP/polling (realtime.md (i)). El umbral de
// frescura es configuración, no constante (ADR 28 §6).
package connection

import (
	"context"
	"time"

	"github.com/AI27-Tech/pf-svc-realtime/internal/stream"
)

// Liveness son los parámetros anunciados al abrir una línea (ADR 28 §6). Son
// configuración operativa, NO constantes del contrato: la raíz los lee de
// config (GetEnvDuration) y los anuncia en el control de línea inicial. Los
// valores de referencia 4 s / 7 s del ADR son defaults de arranque, no se fijan
// aquí.
type Liveness struct {
	// Heartbeat es el intervalo entre latidos.
	Heartbeat time.Duration
	// Timeout es el tiempo sin tráfico tras el cual el cliente considera muerta
	// la línea (liveness_timeout anunciado).
	Timeout time.Duration
	// VentanaReplay es cuánto hacia atrás puede reanudar una línea dentro de su
	// época; fuera de ella, 409 resync (ADR 28 §5-§6).
	VentanaReplay time.Duration
	// FrescuraBus es el tiempo sin evidencia de frescura del bus tras el cual se
	// emite stream.stale (realtime.md (i)). Pendiente de fijar; va a config.
	FrescuraBus time.Duration
}

// Delta es un mensaje listo para escribir a la línea: la capa de dominio opaca
// (de mensajeria) más la posición de transporte que realtime le añadió. El
// bucle no interpreta Dominio; solo lo escribe (realtime.md (b), ADR 28 §8).
type Delta struct {
	Posicion stream.Posicion
	// Dominio es el payload ya autorizado y tipado de mensajeria, serializado.
	// realtime no lo modifica ni lo inspecciona.
	Dominio []byte
}

// Sumidero escribe a la conexión SSE del cliente. Lo implementa el handler HTTP
// sobre http.ResponseWriter + Flusher; el bucle no conoce net/http. Interfaz
// pequeña aceptada por el consumidor (00 regla 6).
type Sumidero interface {
	// EscribirDelta serializa un delta como evento SSE (id, data) y hace flush.
	EscribirDelta(d Delta) error
	// EscribirLatido emite un comentario SSE de latido y hace flush.
	EscribirLatido() error
	// EscribirStale emite el evento de control stream.stale y hace flush.
	EscribirStale() error
}

// Ejecutar corre el bucle de UNA conexión hasta que el cliente se va, la sesión
// se desaloja o el contexto se cancela. Es el select de cuatro ramas que exige
// realtime.md (c):
//
//   - ctx.Done()  → el cliente cerró o el servidor apaga; drena y sale.
//   - desalojo    → la sesión fue revocada o perdió alcance (ADR 26 §6); sale
//     cerrando la línea dentro del SLO.
//   - latido      → emite un latido según Heartbeat.
//   - deltas      → escribe el siguiente delta autorizado al sumidero.
//
// El canal deltas lo alimenta internal/intake tras aplicar targets, prioridad,
// TTL y dedup; aquí solo se entrega. La detección de frescura del bus que
// dispara stream.stale es TODO: su timer entra cuando intake reporte frescura.
func Ejecutar(ctx context.Context, s Sumidero, liveness Liveness, desalojo <-chan struct{}, deltas <-chan Delta) error {
	latido := time.NewTicker(liveness.Heartbeat)
	defer latido.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-desalojo:
			// Sesión revocada o sin alcance: cerrar la línea (ADR 26 §6).
			return nil
		case <-latido.C:
			if err := s.EscribirLatido(); err != nil {
				return err
			}
		case d, ok := <-deltas:
			if !ok {
				// El productor cerró el canal: fin ordenado de la línea.
				return nil
			}
			if err := s.EscribirDelta(d); err != nil {
				return err
			}
		}
	}
	// TODO(realtime): quinta vía de frescura del bus (time.AfterFunc/Timer sobre
	// Liveness.FrescuraBus) que llame s.EscribirStale() al perder evidencia del
	// bus (realtime.md (i)); hoy no hay productor de frescura que la arme.
}
