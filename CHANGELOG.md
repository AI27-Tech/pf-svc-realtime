# CHANGELOG

## 2026-10-28 — tracer.21: subir el pin de contrato

**Resumen:** se sube el pin de `pf-pkg-contracts` de `v0.1.0-tracer.9` a `v0.1.0-tracer.21` (+12 olas, el salto más grande). `realtime` usa solo la superficie SSE mínima congelada en tracer.9 (`GET /api/v1/stream`, `409 stream_resync_required`, DEC-090), que no cambió; el salto es aditivo. Solo `go.mod`/`go.sum`.

**Para quién:** alineación de versión de contrato con el resto del backend. No desbloquea el consumidor `intake` ni los frames de control, que siguen gated por DEC-090 (protocolo SSE sin congelar).

**Cambios:**
- `go.mod`/`go.sum`: `api-client-go` `v0.1.0-tracer.9` → `v0.1.0-tracer.21`.

**Verificación:** `go build/vet ./...`, `go test ./...` (2 paquetes ok), `gofmt` limpio, `arch-test` (v0.1.7) PASS. Sin renombres de enum (§8), pese al salto de 12 olas.

## 2026-10-27 — Alinear arch-test del CI a v0.1.7

**Para quién:** para el equipo. El CI de `realtime` quedó varias versiones atrás del tool de arquitectura (usaba `@v0.1.1`); el resto del backend ya corre `@v0.1.7`. Alinearlo evita que un cambio de regla se valide distinto aquí que en los demás repos.

**Cambios:**
- `.github/workflows/ci.yml`: sube el pin `arch-test@v0.1.1` → `@v0.1.7`.

**Verificado:** `go run .../tools/arch-test@v0.1.7 .` → `arch-test: OK` contra `realtime` (la regla de un `pf-svc-*` normal no cambió entre v0.1.1 y v0.1.7; v0.1.7 solo tocó la regla de `core`).

## 2026-10-22 — Handler SSE GET /api/v1/stream (E0-T9, superficie congelada tracer.9)

**Para quién:** para el cliente que abre la línea de entrega viva de `mensajeria`. El stream SSE ya se puede abrir y se mantiene vivo con latido, autorizado por token; antes `Routes()` no montaba ninguna ruta. Monta la superficie que ADR 28 tiene congelada; la entrega de deltas de dominio reales sigue fuera (intake sin cuerpo, DEC-090).

**Cambios:**
- `adapters/http/stream/stream.go` (nuevo): `Handler` para `GET /api/v1/stream`. Sin `Last-Event-ID` abre línea nueva (stream_id opaco, época 0), emite las cabeceras SSE del contrato (`text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`) y corre el bucle de conexión. Con `Last-Event-ID` responde `409 stream_resync_required` idéntico para toda causa (en V1 no hay registro durable de líneas; ADR 28 §5, sin oráculo). No lee `?canales` (ADR 28 §7).
- `adapters/http/stream/sumidero.go` (nuevo): `sumideroSSE` implementa `connection.Sumidero` sobre `http.ResponseWriter`+`Flusher`: escribe frames SSE (`id`/`data`), latido como comentario, y `stream.stale`. Avanza el seq de transporte por delta; no toca el payload de dominio (ADR 28 §8).
- `realtime.go`: `Routes` monta el handler detrás de `auth.Middleware(Autenticador, authz.Proyeccion)`, que revalida el token y rechaza la sesión revocada en cada apertura. Sin `Autenticador` (standalone) no se monta: una ruta de stream sin verificación sería exponer sin autorizar.

**Alcance / qué NO entra:** la entrega real de deltas (intake), la codificación canónica del id SSE (se usa el shortcut `streamID.epoch.seq` ya presente), los eventos de control de línea más allá del latido, `GET /api/v1/conexiones`, y la emergencia. Todo eso sigue sin congelar fuera de tracer.9.

**Decisión reportada:** ADR 28 §7 dice que el servidor no acepta `?canales`, pero ninguna fuente define si debe **ignorarlo** o **rechazarlo**. El handler lo **ignora** (no lo lee), que respeta "no acepta" sin inventar una regla de error. Pendiente de confirmar con Producto/Arquitectura.

**Verificado:** `go build`/`vet` verde, `gofmt` limpio, `arch-test` OK. Tests (`stream_test.go`): `Last-Event-ID` válido y malformado dan ambos 409 `stream_resync_required`; una línea nueva abre 200 con las tres cabeceras SSE.

## 2026:10:16 17:13

**Resumen:** Scaffold inicial del microservicio `realtime` (E0-T9), el tercer deployable obligatorio de V1 (ADR 29). Superficie pública, puntos de cableado y `TODO`, sin lógica de entrega real. Fija el contrato en `v0.1.0-tracer.9` (superficie SSE mínima, DEC-090) y agrega el flujo de CI del backend.

**Contexto:** sigue el prompt `.scratch/backend-services-scaffold/08-realtime.md`, destrabado al publicarse `tracer.9`. Autoridad: `docs/arquitectura/services/realtime.md`, ADR 26/28/29. `realtime` no usa Postgres (realtime.md (e)): su único almacén es Redis, transitorio. Su única arista síncrona saliente es la restringida a `identidad` para revocación (DEC-051), siempre HTTP; el resto de entradas llega por el bus.

**Cambios:**
- `go.mod`: módulo `github.com/AI27-Tech/pf-svc-realtime`, Go 1.27; `pf-pkg-contracts v0.1.0-tracer.9` y `pf-pkg-platform-go v0.1.5` por versión exacta. Sin pgx (sin Postgres).
- `realtime.go`: `New`, `Routes`, `Workers` (sin `Consultas()`); `Pools` solo con Redis, `Deps` con autenticador JWT, `RevocacionPort`, `Subscriber`, `Broadcaster` y `Liveness`.
- `ports.go`: `RevocacionPort` de dos métodos (`ListarRevocadas`, `ObtenerSesion`), DEC-051.
- `internal/stream`: `StreamID`, `Epoch`, `TransportSeq` como tipos propios separados de cualquier cursor de dominio (P5); codificación/parseo de `Last-Event-ID` y código `stream_resync_required`, con su autochequeo.
- `internal/connection`: bucle `select` de cuatro ramas (cierre, desalojo, latido, delta) sobre `net/http`, sin framework; `Liveness` como configuración anunciada.
- `internal/authz`, `internal/intake`, `internal/sobre`, `internal/replay`: puntos de cableado y pasos documentados, cuerpo `TODO`.
- `adapters/http/revocacion`: adaptador HTTP del `RevocacionPort` sobre el cliente generado de `identidad`, con paginación del snapshot.
- `cmd/api` (standalone, I5) y `cmd/realtime` (raíz de producción con bus en memoria y `Workers()`); `/ready` cableado a la convergencia de tombstones (hoy `TODO` porque `Workers()` está vacío).
- `.github/workflows/`: `ci.yml` (build-test + arch-test, módulos privados por `GH_PAT`, build de `cmd/api` y `cmd/realtime`) y los dos workflows de promoción `d→q` y `q→m`, alineados con los demás servicios Go.
- `tools.go`: fija `arch-test` para `go mod tidy`.

**Archivos modificados:**
- `go.mod`, `go.sum`, `realtime.go`, `ports.go`, `tools.go`, `README.md`
- `internal/{stream,connection,authz,intake,sobre,replay}/`
- `adapters/http/revocacion/revocacion.go`
- `cmd/api/main.go`, `cmd/realtime/main.go`
- `.github/workflows/{ci,promotion-pr-dq,promotion-pr-qm}.yml`

## 2026:10:06 12:55

**Resumen:** Se agregó la plantilla de Pull Request del backend, adaptada al stack Go de AI27.

**Cambios:**
- Se agregó `.github/pull_request_template.md` con secciones de invariantes de arquitectura (I1-I5, contrato primero), base de datos, eventos/bus, verificación (go build/vet/test, arch-test, gofmt), CHANGELOG y checklist de convenciones.

**Archivos modificados:**
- `.github/pull_request_template.md`

## 2026:10:06 12:48

**Resumen:** Se agregó la regla de steering de CHANGELOG y este archivo, antes de que el repo tenga código, para que la convención esté vigente desde el inicio.

**Cambios:**
- Se agregó `.kiro/steering/changelog.md` (`inclusion: always`): toda modificación del repo debe registrarse aquí, en español, con marca de tiempo, resumen, cambios y archivos modificados.
- Se creó `CHANGELOG.md`.

**Archivos modificados:**
- `.kiro/steering/changelog.md`
- `CHANGELOG.md`
