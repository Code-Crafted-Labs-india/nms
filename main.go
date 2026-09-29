package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nms-middleware/alert"
	"nms-middleware/db"
	"nms-middleware/discovery"
	"nms-middleware/handlers"
	"nms-middleware/ping"
	"nms-middleware/topology"
)

type config struct {
	dbDsn string
	env   string
}

type application struct {
	config config
	logger *slog.Logger
	db     *db.DB
}

func main() {
	var cfg config

	flag.StringVar(&cfg.dbDsn, "db-dsn", "postgres://postgres:yoursecurepassword@localhost:5432/nms_db?sslmode=disable", "PostgreSQL connection DSN")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|production)")
	flag.Parse()

	var logger *slog.Logger
	if cfg.env == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	// Boot up connection layer
	database, err := db.InitPool(cfg.dbDsn, logger)
	if err != nil {
		logger.Error("fatal system failure initializing storage pool", "error", err)
		os.Exit(1)
	}

	app := &application{
		config: cfg,
		logger: logger,
		db:     database,
	}

	// 1. Initialize the managed root context to automatically catch OS termination events
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Launch self-healing schema synchronization before any engine loop starts
	if err := db.RunMigrations(ctx, database, logger); err != nil {
		logger.Error("Failed to migrate database", "error", err)
		os.Exit(1)
	}

	app.logger.Info("NMS Go Middleware application initialized successfully. Listening for pipeline updates...")

	// 2. Launch background engine loops
	pingSvc := ping.NewPingEngine(app.db, app.logger)
	go pingSvc.StartSweeper(ctx, 15*time.Second)

	// Fire up the structural network graph mapping package engine
	topology.StartTopologyEngine(ctx, app.db, 30*time.Second)

	// Fire up the auto-discovery engine
	discovery.StartDiscoveryEngine(ctx, database, 1*time.Minute)

	strategies := []alert.AlarmStrategy{
		alert.NewDeviceDownStrategy(logger, 1*time.Minute),
		alert.NewLinkStateStrategy(logger),
		alert.NewAdminOperMismatchStrategy(logger),
		alert.NewHighCPUUtilizationStrategy(logger, 85.0), // Alarm fires if sustained usage surpasses 85%
	}
	logger.Info("Registered operational network alarm matrix successfully", "count", len(strategies))

	alertEngine := alert.NewEvaluateEngine(database, logger, strategies)
	alertEngine.StartWorkerPool(ctx, 5, 30*time.Second)

	// Initialize REST API multiplexer with request logger middleware
	mux := http.NewServeMux()
	mux.Handle("POST /api/devices", handlers.RequestLogger(handlers.AddDeviceHandler(database)))
	mux.Handle("PUT /api/devices/{id}", handlers.RequestLogger(handlers.EditDeviceHandler(database)))
	mux.Handle("DELETE /api/devices/{id}", handlers.RequestLogger(handlers.DeleteDeviceHandler(database)))
	// Explicit OPTIONS routes for preflight on parameterized paths
	mux.Handle("OPTIONS /api/devices", handlers.RequestLogger(handlers.PreflightHandler()))
	mux.Handle("OPTIONS /api/devices/{id}", handlers.RequestLogger(handlers.PreflightHandler()))

	// Binding to :8080 (all interfaces) is required when Grafana runs in a
	// separate Docker container — binding to 127.0.0.1 would be unreachable
	// from any other container on the Docker network.
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	go func() {
		app.logger.Info("Starting REST API server on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Error("REST API server failed", "error", err)
		}
	}()

	app.logger.Info("NMS Backend Middleware is fully operational. Press Ctrl+C to terminate.")

	// 3. Block the main thread right here until an OS signal cancels the context
	<-ctx.Done()

	app.logger.Warn("Caught termination signal. Initializing graceful engine teardown...")

	// 4. Give in-flight HTTP requests and background workers up to 10s to finish
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		app.logger.Error("HTTP server forced to close during shutdown", "error", err)
	} else {
		app.logger.Info("HTTP server shut down cleanly.")
	}

	// 5. Execute safe closure of the active TimescaleDB connection pool
	app.db.Close()
	app.logger.Info("NMS Middleware stopped cleanly. Goodbye.")
}
