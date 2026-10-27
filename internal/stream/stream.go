// Package stream modela la línea SSE resumible: su identidad de transporte
// (stream_id + epoch + seq) y la validación de Last-Event-ID (ADR 28 §4-§5).
//
// Estos tipos son de transporte y SOLO de transporte. El cursor/revisión de
// dominio (conversación: mensaje.secuencia; bandeja: revisión de colección) es
// de mensajeria, viaja opaco dentro de la capa de dominio del sobre, y NO se
// deriva ni se compara con el seq de transporte (realtime.md (b), P5; ADR 28
// §4). Por eso StreamID, Epoch y TransportSeq son tipos propios: el compilador
// impide mezclarlos con cualquier cursor de dominio.
package stream

import (
	"fmt"
	"strconv"
	"strings"
)

// StreamID es el identificador opaco de una línea. No contiene tenant, sujeto,
// bandeja, conversación ni canal en claro (ADR 28 §4): es una etiqueta de
// transporte. La línea queda ligada a su sesión y alcance en internal/authz, no
// en este identificador.
type StreamID string

// Epoch numera las reaperturas de una misma línea. El seq de transporte es
// monotónico DENTRO de una época; al cambiar de época, el seq reinicia.
type Epoch uint64

// TransportSeq es el número de secuencia de transporte, monotónico dentro de una
// línea y época. Sirve solo para reanudar la entrega; no tiene relación con
// ningún cursor de dominio (realtime.md (b)).
type TransportSeq uint64

// Posicion identifica un punto reanudable en una línea: la terna que el campo
// SSE `id` y la cabecera Last-Event-ID codifican (ADR 28 §4). Poseerla NUNCA
// autoriza (ADR 28 §4): la autorización se revalida aparte en cada apertura.
type Posicion struct {
	StreamID StreamID
	Epoch    Epoch
	Seq      TransportSeq
}

// CodificarLastEventID serializa una posición al valor opaco del campo SSE `id`
// y de Last-Event-ID.
//
// shortcut: codificación provisional "streamID.epoch.seq", legible y suficiente
// para reanudar; la codificación canónica (opaca, posiblemente firmada) está
// pendiente de contrato (realtime.md (f), ADR 28 §4). Cambiar cuando el contrato
// la fije. No incluye tenant/sujeto/canal en claro, como exige ADR 28 §4.
func CodificarLastEventID(p Posicion) string {
	return fmt.Sprintf("%s.%d.%d", p.StreamID, p.Epoch, p.Seq)
}

// ParsearLastEventID interpreta el valor de Last-Event-ID enviado por el cliente
// al reconectar. Un valor sintácticamente inválido NO es un error de servidor:
// es un caso de resync. El llamador trata tanto el error como una línea
// desconocida/vencida con la MISMA respuesta 409 stream_resync_required, sin
// distinguir la causa (ADR 28 §5: sin oráculo).
func ParsearLastEventID(valor string) (Posicion, error) {
	partes := strings.Split(valor, ".")
	if len(partes) != 3 || partes[0] == "" {
		return Posicion{}, fmt.Errorf("last-event-id con formato inválido: %q", valor)
	}
	epoch, err := strconv.ParseUint(partes[1], 10, 64)
	if err != nil {
		return Posicion{}, fmt.Errorf("epoch inválido en last-event-id %q: %w", valor, err)
	}
	seq, err := strconv.ParseUint(partes[2], 10, 64)
	if err != nil {
		return Posicion{}, fmt.Errorf("seq inválido en last-event-id %q: %w", valor, err)
	}
	return Posicion{StreamID: StreamID(partes[0]), Epoch: Epoch(epoch), Seq: TransportSeq(seq)}, nil
}

// CodigoResync es el código de máquina de la respuesta 409 cuando una línea no
// puede reanudarse (ADR 28 §5). Es idéntico para todas las causas (línea
// desconocida, de otra sesión, época vencida o fuera de ventana): el servidor no
// revela cuál. El cliente responde repitiendo bootstrap + catch-up HTTP vía bff.
//
// shortcut: constante local porque el cliente generado de tracer.9 no modela el
// stream SSE como operación OpenAPI (DEC-090). Reemplazar por el símbolo
// generado cuando el contrato exponga el código de error del stream.
const CodigoResync = "stream_resync_required"
