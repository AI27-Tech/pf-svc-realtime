// Package authz ata cada línea SSE a una sesión y decide cuándo desalojarla.
//
// Dos responsabilidades, ambas de autorización de transporte (realtime.md (h),
// ADR 26, ADR 28):
//
//   - Verificar el JWT localmente en cada apertura/reconexión y derivar el
//     flujo autorizado del sujeto. Poseer un Last-Event-ID nunca autoriza
//     (ADR 28 §4): la revalidación es por apertura, no por frame.
//   - Mantener la proyección local de invalidaciones (tombstones de sesión,
//     pérdida de alcance, viaje.acceso_revocado) que PREVALECE sobre
//     audience_version y targets anteriores (ADR 28 §9), y desalojar las líneas
//     afectadas dentro del SLO de 30 s (ADR 26 §6).
//
// La proyección se alimenta del snapshot inicial de identidad (ADR 26 §4) y del
// difusor de sesion.revocada (ADR 26 §3). Su cuerpo es TODO en el scaffold: aquí
// va la costura y los tipos, no la lógica de desalojo real.
package authz

import (
	"context"
	"sync"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
)

// Autenticador verifica una cadena de token y devuelve sus claims. Lo cumple el
// *auth.Verifier de producción y el stub de desarrollo (DEC-049), de modo que
// realtime no distinga uno de otro. Es el auth.Authenticator de la plataforma,
// redeclarado aquí para que el consumidor dependa de la interfaz y no del
// paquete concreto (00 regla 6).
type Autenticador interface {
	VerifyToken(ctx context.Context, tokenString string) (*auth.Claims, error)
}

// Proyeccion es la vista local de invalidaciones sobre la que se revalidan y
// desalojan las líneas. El zero value no es usable: constrúyela con
// NewProyeccion. Hasta que converja con el snapshot inicial, la réplica debe
// permanecer not-ready (ADR 26 §4).
type Proyeccion struct {
	autenticador Autenticador

	mu         sync.RWMutex
	convergida bool
	revocadas  map[string]struct{} // sesion_id con tombstone
}

// NewProyeccion crea la proyección con el autenticador que verifica los tokens.
// autenticador puede ser nil en el esqueleto (sin identidad cableada); en ese
// caso Revalidar devolverá siempre no autorizado.
func NewProyeccion(autenticador Autenticador) *Proyeccion {
	return &Proyeccion{
		autenticador: autenticador,
		revocadas:    make(map[string]struct{}),
	}
}

// Convergida indica si la proyección ya cargó el snapshot inicial de tombstones.
// cmd/realtime la usa para no marcar ready hasta converger (ADR 26 §4).
func (p *Proyeccion) Convergida() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.convergida
}

// Revocada indica si una sesión tiene tombstone en la proyección local. Cumple
// la forma de auth.RevocationChecker para que el middleware común la use.
func (p *Proyeccion) Revocada(ctx context.Context, sesionID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.revocadas[sesionID]
	return ok
}

// TODO(realtime): Revalidar(ctx, token, Last-Event-ID) que verifica el JWT,
// deriva el flujo autorizado y rechaza si la sesión está en p.revocadas o perdió
// alcance (ADR 28 §4, §9). Desalojar(sesionID) que cierra todas las líneas de
// una sesión revocada dentro de 30 s (ADR 26 §6). CargarSnapshot(ctx,
// RevocacionPort) que llena p.revocadas y marca p.convergida (ADR 26 §4).
