// Package stream es el adaptador HTTP↔SSE de la línea de entrega viva
// (GET /api/v1/stream, contrato tracer.9, ADR 28). Monta la superficie
// CONGELADA: abrir una línea autorizada, emitir latido, y responder
// 409 stream_resync_required cuando un Last-Event-ID no puede reanudarse. La
// entrega de deltas de dominio reales (intake) sigue sin cuerpo (fuera de
// tracer.9, DEC-090): el canal de deltas existe pero hoy nadie lo alimenta, así
// que la línea abre, late y se mantiene viva sin entregar deltas todavía.
//
// realtime no lee ?canales: el flujo se deriva de la identidad autenticada
// (ADR 28 §7). Qué hacer si el cliente envía ?canales (ignorar vs rechazar) NO
// está decidido en ninguna fuente; aquí se IGNORA, que es lo que "el servidor no
// acepta ?canales" implica sin inventar una regla de error nueva. Reportado.
package stream

import (
	"context"
	"net/http"

	rtclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/realtime"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	"github.com/AI27-Tech/pf-svc-realtime/internal/connection"
	"github.com/AI27-Tech/pf-svc-realtime/internal/stream"
	"github.com/google/uuid"
)

// Handler sirve GET /api/v1/stream. Depende de la forma del bucle de conexión,
// no de net/http para la lógica: el sumidero traduce el bucle a frames SSE.
type Handler struct {
	liveness connection.Liveness
	ejecutar func(ctx context.Context, s connection.Sumidero, liveness connection.Liveness, desalojo <-chan struct{}, deltas <-chan connection.Delta) error
}

// NewHandler construye el handler con los parámetros de liveness anunciados y la
// función de bucle (connection.Ejecutar en producción).
func NewHandler(liveness connection.Liveness, ejecutar func(context.Context, connection.Sumidero, connection.Liveness, <-chan struct{}, <-chan connection.Delta) error) Handler {
	return Handler{liveness: liveness, ejecutar: ejecutar}
}

// Abrir abre la línea SSE. Si llega Last-Event-ID, intenta reanudar; en V1 no
// hay registro durable de líneas, así que toda reanudación es "línea
// desconocida" y responde 409 stream_resync_required (ADR 28 §5): es correcto,
// el cliente repite bootstrap + catch-up HTTP. Sin Last-Event-ID abre una línea
// nueva, emite las cabeceras SSE y corre el bucle hasta que el cliente se va o
// el contexto se cancela.
func (h Handler) Abrir(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Last-Event-ID") != "" {
		// En V1 no hay registro durable de líneas: toda reanudación es
		// irreanudable, sin importar si el valor es válido o malformado. Mismo
		// 409 para toda causa, sin oráculo (ADR 28 §5). Cuando exista el registro
		// de líneas en Redis, aquí se parseará con stream.ParsearLastEventID y se
		// buscará la línea; hoy no hay nada que buscar.
		escribirResync(w)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		// Sin Flusher no hay streaming posible (un proxy que bufferiza romperia
		// SSE). Es un fallo de servidor, no del cliente.
		escribirError(w, platformerrors.NewRoto("realtime: el ResponseWriter no soporta streaming"))
		return
	}

	// Cabeceras SSE (contrato tracer.9): text/event-stream, sin caché, y
	// X-Accel-Buffering: no para que el proxy de la DMZ no bufferice (ADR 32 §2).
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Línea nueva: stream_id opaco, época 0, seq 0 (ADR 28 §4). El id no lleva
	// tenant/sujeto/canal en claro: es un UUID de transporte.
	pos := stream.Posicion{StreamID: stream.StreamID(uuid.NewString()), Epoch: 0, Seq: 0}
	sumidero := &sumideroSSE{w: w, flusher: flusher, pos: pos}

	// El canal de deltas no tiene productor en V1 (intake sin cuerpo, fuera de
	// tracer.9): la línea late y se mantiene viva, pero no entrega deltas de
	// dominio todavía. El desalojo tampoco tiene productor aún (authz sin
	// Desalojar); se cierra por ctx cuando el cliente se va.
	deltas := make(chan connection.Delta)
	desalojo := make(chan struct{})

	// Ejecutar bloquea hasta que el cliente cierra (ctx del request) o el
	// servidor apaga. El error de ctx cancelado es el camino normal de cierre.
	_ = h.ejecutar(r.Context(), sumidero, h.liveness, desalojo, deltas)
}

// escribirResync responde 409 con el codigo de maquina stream_resync_required,
// idéntico para toda causa (ADR 28 §5).
func escribirResync(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = writeJSON(w, rtclient.Error{Codigo: stream.CodigoResync, Mensaje: "la línea no puede reanudarse; repetir bootstrap y catch-up"})
}

// escribirError traduce un PlatformError a su status HTTP con la forma Error del
// contrato (I3).
func escribirError(w http.ResponseWriter, err error) {
	cat := platformerrors.AsCategory(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(cat.HTTPStatus())
	_ = writeJSON(w, rtclient.Error{Codigo: cat.String(), Mensaje: err.Error()})
}
