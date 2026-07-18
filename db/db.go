package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
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

// ============================================================================
// SELF-HEALING SCHEMA SYNCHRONIZATION ENGINE
// ============================================================================

// columnSpec describes the desired state of a single column.
type columnSpec struct {
	name        string
	dataType    string // SQL type for ADD COLUMN, e.g. "TEXT", "TIMESTAMPTZ"
	allowNull   bool   // if false and column has NOT NULL, the constraint is dropped
}

// tableSpec describes the desired state of a single table and its critical columns.
type tableSpec struct {
	name       string
	createDDL  string       // Full CREATE TABLE IF NOT EXISTS statement
	columns    []columnSpec // Columns to inspect/patch after creation
}

// schema is the canonical desired state. Add entries here as the application evolves.
var schema = []tableSpec{
	{
		name: "network_topology",
		createDDL: `CREATE TABLE IF NOT EXISTS network_topology (
			id          SERIAL PRIMARY KEY,
			source_device TEXT,
			source_port   TEXT,
			target_device TEXT,
			target_port   TEXT,
			updated_at    TIMESTAMPTZ
		)`,
		columns: []columnSpec{
			{name: "source_device", dataType: "TEXT", allowNull: true},
			{name: "source_port", dataType: "TEXT", allowNull: true},
			{name: "target_device", dataType: "TEXT", allowNull: true},
			{name: "target_port", dataType: "TEXT", allowNull: true},
			{name: "updated_at", dataType: "TIMESTAMPTZ", allowNull: true},
		},
	},
	{
		// interface_performance_metrics is a hypertable written to by BOTH the Go
		// service (relational FK path) and Telegraf (raw SNMP path with name_override).
		// Telegraf cannot supply 'interface_id', so that NOT NULL constraint must be
		// dropped to allow Telegraf rows to land successfully.
		name: "interface_performance_metrics",
		columns: []columnSpec{
			{name: "interface_id", dataType: "INT", allowNull: true},
		},
	},
	{
		// device_health_metrics hypertable - device_id FK should remain nullable
		// when the Telegraf ICMP plugin writes rows without a resolved device ID.
		name: "device_health_metrics",
		columns: []columnSpec{
			{name: "device_id", dataType: "INT", allowNull: true},
		},
	},
	{
		// snmp_lldp_topology is created dynamically by Telegraf. We ensure the
		// columns our topology engine JOINs on exist with the right types.
		name: "snmp_lldp_topology",
		columns: []columnSpec{
			{name: "lldp_rem_sys_name", dataType: "TEXT", allowNull: true},
			{name: "lldp_rem_port_id", dataType: "TEXT", allowNull: true},
			{name: "lldp_local_port_num", dataType: "TEXT", allowNull: true},
			{name: "agent_host", dataType: "TEXT", allowNull: true},
		},
	},
}

// RunMigrations is the self-healing schema synchronization entry point.
// It inspects the live database state and applies the minimum set of patches
// needed to bring the schema into conformance with the desired state above.
// It is fully idempotent — safe to call on every container startup.
func RunMigrations(ctx context.Context, database *DB, logger *slog.Logger) error {
	logger.Info("[Schema Sync] Starting self-healing schema synchronization pass...")

	for _, table := range schema {
		if err := syncTable(ctx, database, logger, table); err != nil {
			return fmt.Errorf("schema sync failed on table '%s': %w", table.name, err)
		}
	}

	logger.Info("[Schema Sync] All schema checks passed. Database is in the desired state.")
	return nil
}

// syncTable ensures a single table and its required columns conform to spec.
func syncTable(ctx context.Context, database *DB, logger *slog.Logger, spec tableSpec) error {
	exists, err := tableExists(ctx, database, spec.name)
	if err != nil {
		return err
	}

	if !exists {
		if spec.createDDL == "" {
			// Table is Telegraf-managed — we can only patch it once it appears.
			logger.Warn("[Schema Sync] Table not yet created — skipping (will be created by Telegraf)", "table", spec.name)
			return nil
		}
		logger.Info("[Schema Sync] Table missing — creating now", "table", spec.name)
		if _, err := database.Pool.Exec(ctx, spec.createDDL); err != nil {
			return fmt.Errorf("CREATE TABLE failed: %w", err)
		}
		logger.Info("[Schema Sync] Table created successfully", "table", spec.name)
	} else {
		logger.Info("[Schema Sync] Table already exists — skipping creation", "table", spec.name)
	}

	// Patch columns regardless of whether the table was just created or pre-existed.
	for _, col := range spec.columns {
		if err := syncColumn(ctx, database, logger, spec.name, col); err != nil {
			return err
		}
	}

	return nil
}

// syncColumn inspects a single column and applies ADD COLUMN or DROP NOT NULL as needed.
func syncColumn(ctx context.Context, database *DB, logger *slog.Logger, tableName string, col columnSpec) error {
	isNullable, colExists, err := columnInfo(ctx, database, tableName, col.name)
	if err != nil {
		return err
	}

	if !colExists {
		logger.Info("[Schema Sync] Column missing — adding now", "table", tableName, "column", col.name, "type", col.dataType)
		ddl := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", tableName, col.name, col.dataType)
		if _, err := database.Pool.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("ADD COLUMN failed on %s.%s: %w", tableName, col.name, err)
		}
		logger.Info("[Schema Sync] Column added successfully", "table", tableName, "column", col.name)
		return nil
	}

	// Column exists — check if the NOT NULL constraint needs to be relaxed.
	if col.allowNull && !isNullable {
		logger.Warn("[Schema Sync] NOT NULL constraint detected on nullable column — dropping constraint",
			"table", tableName, "column", col.name)
		ddl := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL", tableName, col.name)
		if _, err := database.Pool.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("DROP NOT NULL failed on %s.%s: %w", tableName, col.name, err)
		}
		logger.Info("[Schema Sync] NOT NULL constraint dropped successfully", "table", tableName, "column", col.name)
		return nil
	}

	logger.Info("[Schema Sync] Column already in desired state — no action taken", "table", tableName, "column", col.name)
	return nil
}

// tableExists queries information_schema to check if a table exists in the public schema.
func tableExists(ctx context.Context, database *DB, tableName string) (bool, error) {
	var exists bool
	err := database.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public'
			AND   table_name   = $1
		)
	`, tableName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("tableExists query failed for '%s': %w", tableName, err)
	}
	return exists, nil
}

// columnInfo queries information_schema for a column's existence and nullability.
// Returns (isNullable bool, exists bool, err error).
func columnInfo(ctx context.Context, database *DB, tableName, columnName string) (bool, bool, error) {
	var isNullableStr string
	err := database.Pool.QueryRow(ctx, `
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND   table_name   = $1
		AND   column_name  = $2
	`, tableName, columnName).Scan(&isNullableStr)

	if err == pgx.ErrNoRows {
		return false, false, nil // Column does not exist
	}
	if err != nil {
		return false, false, fmt.Errorf("columnInfo query failed for %s.%s: %w", tableName, columnName, err)
	}

	// information_schema.columns.is_nullable returns 'YES' or 'NO'
	return isNullableStr == "YES", true, nil
}

// ============================================================================
// DATABASE QUERY HELPERS
// ============================================================================

// Close gracefully releases cluster connections
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		db.logger.Info("TimescaleDB connection pool closed safely")
	}
}

func (db *DB) SelectIdAndIp(ctx context.Context) (pgx.Rows, error) {
	res, err := db.Pool.Query(ctx, "SELECT id, ip_address::text FROM devices WHERE is_monitored = true")
	if err != nil {
		return nil, err
	}

	return res, nil
}
