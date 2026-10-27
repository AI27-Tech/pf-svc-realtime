package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/AI27-Tech/pf-svc-realtime/internal/connection"
	intstream "github.com/AI27-Tech/pf-svc-realtime/internal/stream"
)

// sumideroSSE traduce el bucle de conexión a frames SSE sobre el ResponseWriter
// del request. Implementa connection.Sumidero (frontera HTTP: el bucle no conoce
// net/http). Cada delta avanza el seq de transporte de la línea; realtime solo
// añade esa capa, nunca toca el payload de dominio (ADR 28 §8).
type sumideroSSE struct {
	w       http.ResponseWriter
	flusher http.Flusher
	pos     intstream.Posicion
}

var _ connection.Sumidero = (*sumideroSSE)(nil)

// EscribirDelta emite un evento SSE con el id de la línea (stream_id+epoch+seq
// codificado) y el payload de dominio opaco como data. Avanza el seq antes de
// emitir, de modo que cada frame tiene una posición única y reanudable.
func (s *sumideroSSE) EscribirDelta(d connection.Delta) error {
	s.pos.Seq++
	id := intstream.CodificarLastEventID(s.pos)
	if _, err := fmt.Fprintf(s.w, "id: %s\ndata: %s\n\n", id, d.Dominio); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// EscribirLatido emite un comentario SSE (línea que empieza con ':'), que el
// cliente ignora pero mantiene viva la conexión (realtime.md (c), ADR 28 §6).
func (s *sumideroSSE) EscribirLatido() error {
	if _, err := io.WriteString(s.w, ": latido\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// EscribirStale emite el evento de control stream.stale cuando se pierde
// evidencia de frescura del bus (ADR 28 §5): el cliente pasa a recuperación HTTP.
func (s *sumideroSSE) EscribirStale() error {
	if _, err := io.WriteString(s.w, "event: stream.stale\ndata: {}\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// writeJSON serializa v como JSON al writer. Usado por las respuestas de error
// (409, 401, 5xx) antes de abrir el stream.
func writeJSON(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}
