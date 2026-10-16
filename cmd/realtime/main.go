// Command realtime es la RAÍZ DE PRODUCCIÓN del deployable `realtime` (ADR
// 17/29, DEC-050 #2): el mismo cableado HTTP que cmd/api MÁS los Workers()
// embebidos del módulo, arrancados aquí. El readiness del proceso incluye la
// convergencia de la proyección de tombstones de revocación (ADR 26 §4): el
// servicio NO se marca ready hasta cargar el snapshot inicial de identidad.
//
// Scaffolding: cablea Redis (único almacén de realtime), el verificador JWT
// local contra el JWKS del IdP on-premise, el adaptador HTTP a identidad para
// revocación y un bus. El transporte real del broker queda EN ESPERA de ADR 32
// (propuesta: NATS JetStream); mientras tanto se usa el bus en memoria de la
// plataforma (bus/memory), que es Subscriber y Broadcaster, para que Workers()
// tenga transporte cuando el intake y el difusor tengan cuerpo.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/bus"
	"github.com/AI27-Tech/pf-pkg-platform-go/bus/memory"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-pkg-platform-go/observability"
	"github.com/AI27-Tech/pf-pkg-platform-go/redis"
	"github.com/AI27-Tech/pf-pkg-platform-go/server"
	"github.com/AI27-Tech/pf-pkg-platform-go/worker"

	realtime "github.com/AI27-Tech/pf-svc-realtime"
	revocacionhttp "github.com/AI27-Tech/pf-svc-realtime/adapters/http/revocacion"
	"github.com/AI27-Tech/pf-svc-realtime/internal/connection"
)

const (
	envIdentidadBase = "IDENTIDAD_BASE_URL"
	envAudiencia     = "JWT_AUDIENCE"

	// Parámetros de liveness anunciados (ADR 28 §6). Los defaults son los
	// valores de referencia del ADR (4 s / 7 s) más una ventana de replay y un
	// umbral de frescura de arranque; son CONFIGURACIÓN, no constantes del
	// contrato, y se leen de entorno.
	envHeartbeat     = "REALTIME_HEARTBEAT"
	envTimeout       = "REALTIME_LIVENESS_TIMEOUT"
	envVentanaReplay = "REALTIME_REPLAY_WINDOW"
	envFrescuraBus   = "REALTIME_BUS_FRESHNESS"
)

func main() {
	if err := run(); err != nil {
		slog.Error("realtime terminó con error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// realtime requiere Redis (su único almacén), el JWKS para verificar tokens
	// y la URL de identidad para el snapshot de revocación.
	cfg, err := config.Load(config.EnvRedisURL, config.EnvJWKSURL, envIdentidadBase)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("arrancando realtime (raíz de producción)", slog.String("service", cfg.ServiceName))

	shutdownOtel, err := observability.Init(ctx, observability.Config{
		ServiceName: cfg.ServiceName,
		ExporterURL: cfg.OtelExporterURL,
	})
	if err != nil {
		logger.Warn("observabilidad no inicializada", slog.Any("error", err))
	} else {
		defer func() { _ = shutdownOtel(context.Background()) }()
	}

	redisCli, err := redis.New(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer redisCli.Close()

	// Verificador JWT local contra el JWKS del IdP on-premise. URL y audiencia
	// por configuración (realtime.md, cita obligatoria; ADR 32). La revocación
	// efectiva NO es la del IdP, sino el consumo de sesion.revocada (ADR 26).
	verificador := auth.NewVerifier(cfg.JWKSURL, os.Getenv(envAudiencia), 0)
	verificador.IniciarRefrescoProactivo(ctx)

	rev, err := revocacionhttp.NewRevocacionHTTP(os.Getenv(envIdentidadBase), nil)
	if err != nil {
		return err
	}

	// Bus en memoria mientras ADR 32 elige el broker. Es Subscriber y
	// Broadcaster: lo primero para la suscripción durable de sobres, lo segundo
	// para la difusión de sesion.revocada a todas las réplicas (ADR 26 §3).
	busMem := memory.Nuevo(registroPermisivo{}, memory.Opciones{})

	mod, err := realtime.New(
		realtime.Pools{Redis: redisCli},
		realtime.Deps{
			Autenticador: verificador,
			Revocacion:   rev,
			Subscriber:   busMem,
			Broadcaster:  busMem,
			Liveness:     liveness(),
		},
	)
	if err != nil {
		return err
	}

	srv := server.New(cfg.HTTPAddr, server.WithLogger(logger))
	mod.Routes(srv.Mux())
	srv.Registry().RegisterCheck("redis", redisCli.CheckFunc())

	// Arrancar los Workers() embebidos (ADR 29 §6). Su readiness se registra en
	// el mismo health.Registry; mientras la proyección de tombstones no converja
	// (ADR 26 §4), el proceso no debe considerarse listo.
	workersCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	workers := mod.Workers()
	go func() {
		if err := worker.RunGroup(workersCtx, srv.Registry(), workers...); err != nil {
			logger.Error("grupo de workers terminó con error", slog.Any("error", err))
		}
	}()
	logger.Info("workers embebidos arrancados", slog.Int("n", len(workers)))

	// TODO(realtime): marcar ready solo cuando mod expone Convergida() de la
	// proyección de tombstones (ADR 26 §4). Mientras Workers() esté vacío no hay
	// nada que converger, así que el scaffold marca ready para que /ready
	// responda; al cablear el snapshot, condicionar esta llamada a la
	// convergencia.
	srv.MarkReady()
	return srv.Start(ctx)
}

// liveness arma los parámetros anunciados de latido y ventana de replay desde
// entorno, con los valores de referencia de ADR 28 §6 como default. No son
// constantes del contrato.
func liveness() connection.Liveness {
	return connection.Liveness{
		Heartbeat:     config.GetEnvDuration(envHeartbeat, 4*time.Second),
		Timeout:       config.GetEnvDuration(envTimeout, 7*time.Second),
		VentanaReplay: config.GetEnvDuration(envVentanaReplay, 2*time.Minute),
		FrescuraBus:   config.GetEnvDuration(envFrescuraBus, 15*time.Second),
	}
}

// registroPermisivo es el bus.Registry que el bus en memoria exige. En este
// arranque acepta todo y ordena por agregado (la clave de orden de ADR 29). El
// registro REAL se genera desde el contrato y lo inyectará la raíz con el
// adaptador del broker real (en espera de ADR 32); aquí solo basta para que el
// bus arranque.
type registroPermisivo struct{}

func (registroPermisivo) Allowed(string, int) bool       { return true }
func (registroPermisivo) OrderingKey(s bus.Sobre) string { return s.AgregadoID }
