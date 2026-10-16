// Package sobre aplica prioridad, TTL y deduplicación a un sobre vivo según los
// metadatos que el registro versionado declara para su tipo (realtime.md (d),
// ADR 28 §8). NO modifica el payload ni infiere destinatarios: la capa de
// dominio es de mensajeria y viaja opaca.
//
// El sobre de dos capas: la capa de dominio (identificador, tipo versionado,
// agregado, cursor, audience_version, target_sujeto_ids, payload) es de
// mensajeria; realtime solo añade la capa de transporte (stream_id, epoch, seq).
// El sobre plano anterior quedó eliminado (ADR 28 §8). Los nombres y valores de
// los tipos de la familia de mensajeria NO se fijan aquí: su registro no está
// congelado en el contrato (fuera de tracer.9, DEC-090).
package sobre

// TODO(realtime): aplicar prioridad/TTL/dedup declaradas por tipo cuando el
// registro versionado de mensajeria esté contratado. Hoy no hay tipos que
// registrar, así que no hay nada que aplicar.
