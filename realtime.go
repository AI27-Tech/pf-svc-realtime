// Package realtime mantiene las conexiones SSE abiertas y entrega sobres ya
// autorizados por ellas, con secuencia de transporte, prioridad, TTL,
// deduplicación y latido — sin saber qué significa el contenido (realtime.md
// (a)). Es el ***cómo*** de la separación cuándo/qué/cómo: no decide cuándo
// publicar (lo decide mensajeria) ni qué (el payload es de mensajeria).
//
// realtime no es dueño de ningún dato durable (realtime.md (e)): su único
// almacén es Redis, transitorio (registro de línea, ventana de replay,
// tombstones/invalidaciones). Por eso este módulo no recibe pools de Postgres.
package realtime

import (
	"net/http"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/bus"
	"github.com/AI27-Tech/pf-pkg-platform-go/redis"
	"github.com/AI27-Tech/pf-pkg-platform-go/worker"

	streamhttp "github.com/AI27-Tech/pf-svc-realtime/adapters/http/stream"
	"github.com/AI27-Tech/pf-svc-realtime/internal/authz"
	"github.com/AI27-Tech/pf-svc-realtime/internal/connection"
)

// Pools agrupa los almacenes que el módulo necesita. A diferencia de los demás
// servicios, realtime NO tiene pools de Postgres (realtime.md (e)): su único
// almacén es Redis, y todo lo que guarda ahí es transitorio. Es la desviación
// justificada de la forma de 00 (tabla de servicios: «ninguno (solo Redis)»).
type Pools struct {
	// Redis es el almacén transitorio: registro de línea, ventana de replay,
	// lectura de sesion:{sesion_id} (lo escribe identidad; realtime solo lo usa
	// como acelerador) y tombstones/invalidaciones. Puede ser nil en el
	// arranque standalone (cmd/api) sin Redis.
	Redis *redis.Client
}

// Deps agrupa los puertos de salida y la configuración que la raíz resuelve por
// el consumidor. New no lee entorno (00 regla 4): la raíz cablea todo.
type Deps struct {
	// Autenticador verifica el JWT localmente contra el JWKS del IdP on-premise
	// (realtime.md, cita obligatoria; ADR 32). La revocación efectiva NO es la
	// del IdP sino el consumo de sesion.revocada (ADR 26). Puede ser nil en el
	// esqueleto; sin él authz no puede atar la línea a una sesión.
	Autenticador authz.Autenticador
	// Revocacion es la arista restringida a identidad (DEC-051): snapshot de
	// tombstones al arrancar y consulta puntual en modo degradado. Opcional en
	// el esqueleto.
	Revocacion RevocacionPort
	// Subscriber es el bus durable del que realtime consume los sobres vivos de
	// mensajeria (tema restringido) y viaje.acceso_revocado. Nil sin bus:
	// Workers() no arranca consumidores.
	Subscriber bus.Subscriber
	// Broadcaster es el fan-out sin durabilidad de sesion.revocada a todas las
	// réplicas (ADR 26 §3, ADR 29 §3): cada réplica desaloja sus líneas del
	// sesion_id. Nil sin bus: Workers() no arranca el difusor.
	Broadcaster bus.Broadcaster
	// Liveness son los parámetros anunciados de latido y ventana de replay
	// (ADR 28 §6). Son configuración, no constantes del contrato: la raíz los
	// lee de config.
	Liveness connection.Liveness
}

// Module es la instancia construida del módulo realtime.
type Module struct {
	pools Pools
	deps  Deps
	authz *authz.Proyeccion
}

// New recibe el almacén Redis y los puertos ya resueltos. No lee entorno
// (00 regla 4): la raíz cablea todo. Construye la proyección de autorización
// (tombstones + invalidaciones) sobre la que se apoyan las líneas; su carga
// inicial desde identidad es un worker, no trabajo de construcción.
func New(p Pools, d Deps) (*Module, error) {
	return &Module{
		pools: p,
		deps:  d,
		authz: authz.NewProyeccion(d.Autenticador),
	}, nil
}

// Routes monta las rutas HTTP del módulo bajo /api/v1/... en el mux dado.
//
// Monta GET /api/v1/stream (superficie SSE congelada de tracer.9, ADR 28)
// detrás de auth.Middleware: cada apertura revalida el token y rechaza la sesión
// revocada contra la proyección local de tombstones (m.authz, que es
// RevocationChecker). El handler deriva el flujo del sujeto autenticado; no lee
// ?canales (ADR 28 §7). La entrega de deltas reales (intake) sigue sin cuerpo
// fuera de tracer.9 (DEC-090): la línea abre, late y se mantiene viva.
//
// Sin Autenticador (esqueleto standalone) NO se monta: una ruta de stream sin
// verificación de token sería exponer sin autorizar.
func (m *Module) Routes(mux *http.ServeMux) {
	if m.deps.Autenticador == nil {
		return
	}
	h := streamhttp.NewHandler(m.deps.Liveness, connection.Ejecutar)
	protegido := auth.Middleware(m.deps.Autenticador, m.authz)(http.HandlerFunc(h.Abrir))
	mux.Handle("GET /api/v1/stream", protegido)
}

// Workers devuelve los workers embebidos del módulo (ADR 29 §6). La raíz de
// producción (cmd/realtime) decide arrancarlos; cmd/api (standalone) no.
//
// En V1, cuando la raíz inyectó el bus, realtime arranca:
//   - el consumidor de la suscripción durable propia (DEC-057) del tema
//     restringido: sobres vivos de mensajeria y viaje.acceso_revocado;
//   - el consumidor-difusor de sesion.revocada que invalida en todas las
//     réplicas (Broadcaster, ADR 26 §3);
//   - la carga inicial del snapshot de tombstones desde identidad, que mantiene
//     al módulo not-ready hasta converger (ADR 26 §4).
//
// Sus efectos de dominio están sin implementar en el scaffold, así que todavía
// no se abren suscripciones que no apliquen nada: devuelve vacío hasta que el
// intake y el difusor tengan cuerpo.
func (m *Module) Workers() []worker.Worker {
	// TODO(realtime): devolver el consumidor del tema restringido (Subscriber),
	// el difusor de sesion.revocada (Broadcaster) y el cargador de snapshot
	// (Revocacion) cuando internal/intake y internal/authz apliquen efectos.
	return nil
}

// NOTA — SIN Consultas(): 00 dice que Consultas() existe solo si el módulo es
// destino de una arista síncrona ENTRANTE EN PROCESO. Ningún módulo llama a
// realtime en proceso: el stream SSE llega por HTTP directo (no a través de
// bff), y realtime no co-ubica con nadie. Por eso no expone Consultas()
// (realtime.md (f); igual que mensajeria).
