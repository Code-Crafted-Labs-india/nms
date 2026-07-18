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

	app.logger.Info("NMS Go Middleware application initialized successfully. Listening for pipeline updates...")

	// 2. Launch background engine loops
	pingSvc := ping.NewPingEngine(app.db, app.logger)
	go pingSvc.StartSweeper(ctx, 15*time.Second)

	// Fire up the structural network graph mapping package engine
	topology.StartTopologyEngine(app.db, 30*time.Second)

	// Fire up the auto-discovery engine
	discovery.StartDiscoveryEngine(database, 1*time.Minute)

	strategies := []alert.AlarmStrategy{
		alert.NewDeviceDownStrategy(logger, 1*time.Minute),
		alert.NewLinkStateStrategy(logger),
		alert.NewAdminOperMismatchStrategy(logger),
		alert.NewHighCPUUtilizationStrategy(logger, 85.0), // Alarm fires if sustained usage surpasses 85%
	}
	logger.Info("Registered operational network alarm matrix successfully", "count", len(strategies))

	engine := alert.NewEvaluateEngine(database, logger, strategies)
	engine.StartWorkerPool(ctx, 5, 30*time.Second)

	// Initialize REST API multiplexer
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/devices", handlers.AddDeviceHandler(database))
	mux.HandleFunc("PUT /api/devices/{id}", handlers.EditDeviceHandler(database))
	mux.HandleFunc("DELETE /api/devices/{id}", handlers.DeleteDeviceHandler(database))

	go func() {
		app.logger.Info("Starting REST API server on :8080")
		if err := http.ListenAndServe(":8080", mux); err != nil && err != http.ErrServerClosed {
			app.logger.Error("REST API server failed", "error", err)
		}
	}()

	app.logger.Info("NMS Backend Middleware is fully operational. Press Ctrl+C to terminate.")

	// 3. Block the main thread right here until an OS signal cancels the context
	<-ctx.Done()

	app.logger.Warn("Caught termination signal. Initializing graceful engine teardown...")

	// 4. Provide a brief 2-second buffer loop to allow workers to clear out active SQL commands
	time.Sleep(2 * time.Second)

	// 5. Execute safe closure of the active TimescaleDB connection pool
	app.db.Close()
	app.logger.Info("NMS Middleware stopped cleanly. Goodbye.")
}
