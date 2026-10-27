// Package revocacion contiene el adaptador HTTP del puerto RevocacionPort de
// realtime (cliente generado de identidad). realtime e identidad viven en
// deployables distintos, así que esta arista es SIEMPRE HTTP: no existe
// adaptador en proceso (DEC-051, realtime.md (f)). Lo usan cmd/api y
// cmd/realtime por igual.
package revocacion

import (
	"context"
	"fmt"
	"net/http"

	idclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/identidad"
	realtime "github.com/AI27-Tech/pf-svc-realtime"
)

// RevocacionHTTP implementa realtime.RevocacionPort con el cliente generado de
// identidad. Cubre el snapshot inicial de tombstones (ADR 26 §4) y la consulta
// puntual en modo degradado (ADR 26 §5).
type RevocacionHTTP struct {
	cli *idclient.ClientWithResponses
}

var _ realtime.RevocacionPort = (*RevocacionHTTP)(nil)

// NewRevocacionHTTP crea el adaptador apuntando a la URL base de identidad.
func NewRevocacionHTTP(baseURL string, doer idclient.HttpRequestDoer) (*RevocacionHTTP, error) {
	opts := []idclient.ClientOption{}
	if doer != nil {
		opts = append(opts, idclient.WithHTTPClient(doer))
	}
	cli, err := idclient.NewClientWithResponses(baseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("realtime: crear cliente de identidad: %w", err)
	}
	return &RevocacionHTTP{cli: cli}, nil
}

// ListarRevocadas recorre la paginación de /sesiones/revocadas y devuelve todos
// los sesion_id con tombstone vigente, para el snapshot inicial (ADR 26 §4). Se
// detiene cuando identidad deja de enviar cursor_siguiente.
func (a *RevocacionHTTP) ListarRevocadas(ctx context.Context) ([]string, error) {
	var ids []string
	var cursor *idclient.Cursor
	for {
		ctxPag, cancel := context.WithTimeout(ctx, realtime.PresupuestoRevocacion)
		resp, err := a.cli.ListarSesionesRevocadasWithResponse(ctxPag, &idclient.ListarSesionesRevocadasParams{Cursor: cursor})
		cancel()
		if err != nil {
			return nil, fmt.Errorf("realtime: listar sesiones revocadas: %w", err)
		}
		if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
			return nil, fmt.Errorf("realtime: listar sesiones revocadas: estado %d", resp.StatusCode())
		}
		for _, r := range resp.JSON200.Items {
			ids = append(ids, r.SesionId.String())
		}
		if resp.JSON200.CursorSiguiente == nil || *resp.JSON200.CursorSiguiente == "" {
			return ids, nil
		}
		sig := idclient.Cursor(*resp.JSON200.CursorSiguiente)
		cursor = &sig
	}
}

// ObtenerSesion consulta el estado durable de UNA sesión en modo degradado, tras
// 15 s sin frescura del canal (ADR 26 §5). Una sesión inexistente (404) no es un
// error: se trata como no revocada, y el llamador decide según su propia
// política de fallo.
func (a *RevocacionHTTP) ObtenerSesion(ctx context.Context, sesionID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, realtime.PresupuestoRevocacion)
	defer cancel()

	id, err := uuidDesde(sesionID)
	if err != nil {
		return false, fmt.Errorf("realtime: obtener sesión: %w", err)
	}
	resp, err := a.cli.ObtenerSesionWithResponse(ctx, id)
	if err != nil {
		return false, fmt.Errorf("realtime: obtener sesión: %w", err)
	}
	switch {
	case resp.StatusCode() == http.StatusOK && resp.JSON200 != nil:
		return resp.JSON200.Revocada, nil
	case resp.StatusCode() == http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("realtime: obtener sesión: estado %d", resp.StatusCode())
	}
}

// uuidDesde parsea el sesion_id al tipo Id (UUID) que exige el cliente generado.
func uuidDesde(s string) (idclient.Id, error) {
	var id idclient.Id
	if err := id.UnmarshalText([]byte(s)); err != nil {
		return id, fmt.Errorf("sesion_id no es UUID válido: %w", err)
	}
	return id, nil
}
