package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB encapsulates our TimescaleDB connection pool worker
type DB struct {
	Pool   *pgxpool.Pool
	logger *slog.Logger
}

// InitPool initializes an optimized connection pool
func InitPool(connString string, logger *slog.Logger) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse configuration details
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	// Optimize connection parameters for high-throughput network monitoring
	config.MaxConns = 25                      // Prevent overloading DB container
	config.MinConns = 5                       // Keep cold standbys open
	config.MaxConnLifetime = 30 * time.Minute // Cycle old connections
	config.MaxConnIdleTime = 5 * time.Minute  // Clean up unused resources

	// Instantiate connection pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Verify the database engine is responsive
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database unreachable on ping: %w", err)
	}

	logger.Info("Successfully connected to TimescaleDB connection pool")
	return &DB{Pool: pool, logger: logger}, nil
}

// Close gracefully releases cluster connections
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		db.logger.Info("TimescaleDB connection pool closed safely")
	}
}