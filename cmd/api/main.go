// Command api es el arranque STANDALONE del servicio realtime (I5): compila y
// arranca solo, sirviendo /health y /ready, sin bus ni workers. No arranca
// Workers() (eso es cmd/realtime). Sirve para probar el binario y el cableado
// HTTP sin dependencias de infraestructura.
//
// A diferencia de los demás servicios, realtime NO abre Postgres (realtime.md
// (e)): su único almacén es Redis, y en standalone puede ir sin él.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-pkg-platform-go/observability"
	"github.com/AI27-Tech/pf-pkg-platform-go/server"
	realtime "github.com/AI27-Tech/pf-svc-realtime"
)

func main() {
	if err := run(); err != nil {
		slog.Error("realtime terminó con error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Standalone no requiere ninguna URL: Redis, JWKS y el bus los cablea
	// cmd/realtime. config.Load sin requeridas da defaults de HTTP_ADDR.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("arrancando realtime (standalone)", slog.String("service", cfg.ServiceName))

	shutdownOtel, err := observability.Init(ctx, observability.Config{
		ServiceName: cfg.ServiceName,
		ExporterURL: cfg.OtelExporterURL,
	})
	if err != nil {
		logger.Warn("observabilidad no inicializada", slog.Any("error", err))
	} else {
		defer func() { _ = shutdownOtel(context.Background()) }()
	}

	// Sin Redis, sin autenticador, sin bus: el esqueleto arranca para servir
	// /health y /ready. Routes() no monta nada todavía.
	mod, err := realtime.New(realtime.Pools{}, realtime.Deps{})
	if err != nil {
		return err
	}

	srv := server.New(cfg.HTTPAddr, server.WithLogger(logger))
	mod.Routes(srv.Mux())

	// Standalone no consume sesion.revocada, así que no hay proyección de
	// tombstones que converger: marca ready de inmediato.
	srv.MarkReady()
	return srv.Start(ctx)
}
