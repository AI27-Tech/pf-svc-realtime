// Package intake consume la suscripción durable propia de realtime (DEC-057):
// los sobres vivos de mensajeria por el tema restringido y viaje.acceso_revocado
// de operacion (realtime.md (g), ADR 29). Valida tipo/versión contra el registro
// versionado; un tipo o versión no registrados es evento venenoso y se confirma
// en cuarentena ANTES del fanout (ADR 29 §4).
//
// realtime NO interpreta el payload ni infiere destinatarios: toma los
// target_sujeto_ids explícitos del sobre (ADR 28 §8). El cuerpo del consumidor
// es TODO en el scaffold; aquí van el puerto y los pasos documentados del delta.
package intake

// Procesamiento de un delta V1 (realtime.md (d)), que el consumidor hará cuando
// tenga cuerpo:
//
//  1. Validar tipo y versión contra el registro compartido; no registrado →
//     cuarentena antes del fanout (ADR 29 §4).
//  2. Tomar los target_sujeto_ids explícitos; NO consultar membresía.
//  3. Aplicar invalidaciones conocidas (revocación, pérdida de alcance,
//     viaje.acceso_revocado): prevalecen sobre audience_version y targets
//     anteriores (ADR 28 §9).
//  4. Añadir solo la capa de transporte (stream_id, epoch, seq) y aplicar la
//     prioridad/TTL/dedup declaradas por el tipo.
//  5. Conservar el delta en la ventana de replay y escribirlo a las líneas
//     autorizadas.
//
// TODO(realtime): Consumidor que implementa bus.Handler sobre la suscripción
// restringida y ejecuta estos cinco pasos, más el difusor de sesion.revocada
// sobre bus.Broadcaster (ADR 26 §3). Esperan a que authz aplique desalojos y a
// que el registro versionado de tipos de mensajeria esté congelado en el
// contrato (hoy fuera de tracer.9, DEC-090).
