package realtime

import (
	"context"
	"time"
)

// RevocacionPort es la única arista síncrona saliente de realtime: la consulta
// restringida a identidad para revocación de sesiones (DEC-051, realtime.md
// (f)). Siempre es HTTP (realtime es deployable propio, no co-ubica con
// identidad); su adaptador vive en adapters/http/. El consumidor declara la
// interfaz (00 regla 6, "accept interfaces").
//
// La difusión interna de sesion.revocada a todas las réplicas (ADR 26 §3) es un
// mecanismo APARTE, alimentado por el Broadcaster del bus, no por este puerto:
// este puerto es solo el snapshot inicial y el fallback degradado.
type RevocacionPort interface {
	// ListarRevocadas devuelve las sesiones revocadas vigentes para el snapshot
	// inicial al arrancar. La réplica permanece not-ready hasta cargarlo y
	// converger (ADR 26 §4).
	ListarRevocadas(ctx context.Context) ([]string, error)
	// ObtenerSesion consulta una sesión puntual en modo degradado, tras 15 s sin
	// evidencia de frescura del canal (ADR 26 §5).
	ObtenerSesion(ctx context.Context, sesionID string) (revocada bool, err error)
}

// PresupuestoRevocacion es el presupuesto de tiempo de UNA llamada puntual de la
// arista restringida a identidad (snapshot o fallback), no el umbral de frescura
// del canal (15 s / corte 30 s, que fija ADR 26 §5-§6). 2 s es el default común
// en los servicios; no es un número de negocio aprobado.
const PresupuestoRevocacion = 2 * time.Second
