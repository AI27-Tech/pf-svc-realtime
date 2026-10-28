module github.com/AI27-Tech/pf-svc-realtime

go 1.27

// realtime es capa de soporte y vive en su propio deployable (00, tabla de
// topología; realtime.md (c), ADR 17, ADR 29). Depende SOLO de los paquetes
// publicados, nunca de otro pf-svc-* (00 regla 1). Sin replace a rutas locales
// ni go.work versionado (00 regla 7).
//
// A diferencia de los demás servicios, realtime NO usa Postgres (realtime.md
// (e)): sin pgx, sin migraciones, sin pool. Su único almacén es Redis, todo
// transitorio.
//
// Aristas salientes: solo la restringida a identidad para revocación
// (listarRevocadas, obtenerSesion), siempre HTTP vía cliente generado
// (DEC-051). No requiere el módulo identidad, solo su cliente generado del
// contrato.
//
// Versión de contrato fijada por el prompt 08 (DEC-090): la superficie SSE
// mínima de realtime entró en contracts v0.1.0-tracer.9.
require (
	github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go v0.1.0-tracer.21
	github.com/AI27-Tech/pf-pkg-platform-go v0.1.5
	github.com/google/uuid v1.6.0
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/oapi-codegen/runtime v1.7.0 // indirect
	github.com/redis/go-redis/v9 v9.22.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.46.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.46.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/grpc v1.83.1 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
