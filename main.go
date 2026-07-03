package main

import (
	"context"
	"flag"
	"log/slog"
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

	ctx, cancel := context.WithCancel(context.Background())

	app.logger.Info("NMS Go Middleware application initialized successfully. Listening for pipeline updates...")

	// Spin up the background pinger loop service
	pingSvc := ping.NewPingEngine(app.db, app.logger)
	go pingSvc.StartSweeper(ctx, 15*time.Second) // Poll inventory targets every 15 seconds
	// Graceful shutdown channel
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	s := <-quit
	app.logger.Warn("caught termination signal", "signal", s.String())

	cancel()

	// Execute clean closure of the active TimescaleDB connection pool
	app.db.Close()

	// Exit cleanly
	os.Exit(0)
}
