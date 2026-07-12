package main

import (
	"context"
	"flag"
	"log/slog"

	// "nms-middleware/alert"
	"nms-middleware/alert"
	"nms-middleware/db"
	"nms-middleware/ping"
	"os"
	"os/signal"
	"syscall"
	"time"
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

	strategies := []alert.AlarmStrategy{
		alert.NewDeviceDownStrategy(logger, 1*time.Minute),
		alert.NewLinkStateStrategy(logger),
		alert.NewAdminOperMismatchStrategy(logger),
		alert.NewHighCPUUtilizationStrategy(logger, 85.0), // Alarm fires if sustained usage surpasses 85%
	}
	logger.Info("Registered operational network alarm matrix successfully", "count", len(strategies))

	engine := alert.NewEvaluateEngine(database, logger, strategies)
	engine.StartWorkerPool(ctx, 5, 30*time.Second)

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
