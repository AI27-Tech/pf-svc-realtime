// Package replay mantiene la ventana corta de deltas por línea y época en Redis,
// para reanudar una línea tras una reconexión dentro de la ventana (realtime.md
// (e), ADR 28 §5-§6). Fuera de la ventana, el cliente debe hacer resync HTTP:
// no hay replay general del broker en V1 (ADR 29 §5).
//
// La retención es "de segundos a minutos" (realtime.md (a)) y es configurable y
// anunciada, no una constante. Todo lo que guarda es transitorio: ninguna
// fuente de verdad vive aquí.
package replay

// TODO(realtime): Ventana sobre redis.Client con Guardar(linea, Delta) y
// Reanudar(Posicion) acotados por la ventana configurada; fuera de ventana,
// señalar resync. Espera a que internal/connection entregue deltas reales.
