package stream_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rtclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/realtime"
	streamhttp "github.com/AI27-Tech/pf-svc-realtime/adapters/http/stream"
	"github.com/AI27-Tech/pf-svc-realtime/internal/connection"
)

// bucleInmediato es un doble de connection.Ejecutar que no bloquea: simula que
// el cliente se fue de inmediato (contexto cancelado), para que el handler emita
// las cabeceras SSE y vuelva sin colgar el test.
func bucleInmediato(ctx context.Context, s connection.Sumidero, l connection.Liveness, desalojo <-chan struct{}, deltas <-chan connection.Delta) error {
	return nil
}

func nuevoHandler() streamhttp.Handler {
	return streamhttp.NewHandler(connection.Liveness{Heartbeat: time.Second}, bucleInmediato)
}

// TestLastEventIDDaResync afirma que CUALQUIER Last-Event-ID produce
// 409 stream_resync_required en V1 (no hay registro durable de líneas, así que
// toda reanudación es irreanudable; ADR 28 §5). El código es idéntico sin
// importar si el valor es válido o malformado: sin oráculo.
func TestLastEventIDDaResync(t *testing.T) {
	h := nuevoHandler()

	for _, lastID := range []string{"abc.1.5", "basura-invalida", "x"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
		req.Header.Set("Last-Event-ID", lastID)
		rec := httptest.NewRecorder()
		h.Abrir(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("Last-Event-ID %q: status %d, quiere 409; cuerpo %s", lastID, rec.Code, rec.Body.String())
		}
		var e rtclient.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
			t.Fatalf("decodificar Error: %v; cuerpo %s", err, rec.Body.String())
		}
		if e.Codigo != "stream_resync_required" {
			t.Fatalf("Last-Event-ID %q: codigo %q, quiere stream_resync_required", lastID, e.Codigo)
		}
	}
}

// TestLineaNuevaAbreConCabecerasSSE afirma que una conexión sin Last-Event-ID
// abre con 200 y las cabeceras SSE del contrato (text/event-stream, no-cache,
// X-Accel-Buffering: no). El bucle inmediato hace que el handler vuelva tras
// emitir las cabeceras.
func TestLineaNuevaAbreConCabecerasSSE(t *testing.T) {
	h := nuevoHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
	rec := httptest.NewRecorder()
	h.Abrir(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("línea nueva: status %d, quiere 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, quiere text/event-stream", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, quiere no-cache", cc)
	}
	if ab := rec.Header().Get("X-Accel-Buffering"); ab != "no" {
		t.Fatalf("X-Accel-Buffering = %q, quiere no", ab)
	}
}
