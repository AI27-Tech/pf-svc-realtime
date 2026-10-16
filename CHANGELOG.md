# CHANGELOG

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
