# pf-svc-realtime

Servicio `realtime` de AI27: mantiene las conexiones SSE abiertas y **entrega
sobres ya autorizados** por ellas —con secuencia de transporte, prioridad, TTL,
deduplicación y latido— **sin saber qué significa el contenido**. Es el *cómo*
de la separación cuándo/qué/cómo (ADR 28); el *cuándo* y el *qué* los decide
`mensajeria`.

Tercer deployable obligatorio de V1 junto a `core` y `mensajeria` (ADR 29).

Autoridad de diseño: `docs/arquitectura/services/realtime.md` y ADR 26, 28, 29.
No reabrir decisiones cerradas ahí.

## Estado

**Scaffold inicial (E0-T9).** Superficie pública, puntos de cableado y `TODO`;
todavía no entrega deltas reales. El contrato está fijado en
`v0.1.0-tracer.9` (superficie SSE mínima: `GET /api/v1/stream` y el
`409 stream_resync_required`, DEC-090). Lo que sigue fuera del contrato
congelado —codificación del `id`/`Last-Event-ID`, eventos de control de línea
(`stream.stale`), registro versionado de tipos de `mensajeria`, emergencia— está
marcado como `TODO` y espera una ola contractual futura.

## Forma del módulo

- `realtime.go` — `New`, `Routes`, `Workers`. Sin `Consultas()`: ningún módulo
  llama a `realtime` en proceso. `Pools` lleva solo Redis (sin Postgres,
  realtime.md (e)).
- `ports.go` — `RevocacionPort`: la única arista síncrona saliente, hacia
  `identidad`, siempre HTTP (DEC-051).
- `internal/stream` — línea resumible: `StreamID`, `Epoch`, `TransportSeq` (tipos
  de transporte, nunca mezclados con un cursor de dominio) y validación de
  `Last-Event-ID`.
- `internal/connection` — una gorutina por conexión; `select` de cuatro ramas
  (cierre del cliente, desalojo, latido, delta) sobre `net/http`, sin framework.
- `internal/authz` — verificación JWT local, proyección de tombstones e
  invalidaciones, desalojo (ADR 26/28).
- `internal/intake` — consumo de la suscripción durable del tema restringido;
  valida tipo/versión, cuarentena antes del fanout (ADR 29).
- `internal/sobre`, `internal/replay` — prioridad/TTL/dedup declarados y ventana
  de replay corta en Redis.
- `adapters/http/revocacion` — adaptador del `RevocacionPort` sobre el cliente
  generado de `identidad`.
- `cmd/api` — arranque standalone (I5): `/health`, `/ready`, sin bus.
- `cmd/realtime` — raíz de producción: Redis, verificador JWT, bus y `Workers()`
  embebidos. El broker real espera ADR 32; mientras tanto, bus en memoria.

## Convención de idioma

Prosa y comentarios en español (México); identificadores en inglés salvo los
sustantivos de dominio (`sobre`, `sesion`, `viaje`, `Audiencia`), que conservan
su forma canónica en español. Ver `.kiro/steering/naming-conventions.md`.

## Verificación

```sh
go build ./...
go vet ./...
go test ./...
go build ./cmd/api ./cmd/realtime
go run github.com/AI27-Tech/pf-pkg-platform-go/tools/arch-test@v0.1.1 .
```
