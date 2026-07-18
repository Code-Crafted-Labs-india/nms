package alert

import (
	"context"
	"errors"
	"log/slog"
	"nms-middleware/db"
	"time"

	"github.com/jackc/pgx/v5"
)

// DeviceDownStrategy checks if pings are failing consistently over a window
type DeviceDownStrategy struct {
	logger         *slog.Logger
	windowDuration time.Duration
}

// NewDeviceDownStrategy initializes our specific rule structure
func NewDeviceDownStrategy(logger *slog.Logger, window time.Duration) *DeviceDownStrategy {
	return &DeviceDownStrategy{
		logger:         logger,
		windowDuration: window,
	}
}

func (d *DeviceDownStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	// Step 1 & 2: Query the rolling 1-minute metric matrix window
	query := `
        SELECT COUNT(*), COALESCE(SUM(icmp_status), 0) 
        FROM device_health_metrics 
        WHERE device_id = $1 AND time >= NOW() - INTERVAL '1 minute';`

	var count int
	var statusSum int

	err := database.Pool.QueryRow(ctx, query, deviceID).Scan(&count, &statusSum)
	if err != nil {
		d.logger.Error("Failed to evaluate sliding window metrics", "device_id", deviceID, "err", err)
		return err
	}

	// Handle our "Dead-Man's Switch" silence condition
	if count == 0 {
		d.logger.Warn("Telemetry silence detected! No metrics found.", "device_id", deviceID)
		// You could trigger a distinct "Missing Telemetry" alarm here if desired
		return nil
	}

	// If sum of icmp_status > 0, at least one check succeeded -> Device is UP, nothing to do
	if statusSum > 0 {
		return nil
	}

	// Step 3: Read-Before-Write state check to prevent duplicate alarm spam
	// Alarm ID 1 represents our "Device Down" rule condition signature
	const alarmID = 1
	checkQuery := `
        SELECT 1 FROM network_events 
        WHERE device_id = $1 AND alarm_id = $2 AND resolved = false 
        LIMIT 1;`

	var placeholder int
	err = database.Pool.QueryRow(ctx, checkQuery, deviceID, alarmID).Scan(&placeholder)

	// If pgx.ErrNoRows is returned, it means no active alert is open -> Time to write!
	if err != nil {
		d.logger.Warn("Critical threshold breach! Registering fresh outage event.", "device_id", deviceID)

		insertQuery := `
            INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, details, resolved) 
            VALUES (NOW(), $1, $2, 'device_down', 'CRITICAL', 'Device failed to respond to concurrent ICMP sweeps over 60s window.', false);`

		_, insertErr := database.Pool.Exec(ctx, insertQuery, deviceID, alarmID)
		if insertErr != nil {
			d.logger.Error("Failed to commit network event log", "device_id", deviceID, "err", insertErr)
			return insertErr
		}
	} else {
		// An active unresolved row was found, so we suppress duplicate notification writes
		d.logger.Info("Active unresolved outage already registered. Suppressing duplicate alert log.", "device_id", deviceID)
	}

	return nil
}

// =========================================================================
// STRATEGY 2: LinkStateStrategy (Alarm 2)
// Tracks if an interface has physically dropped (ifOperStatus = 2)
// =========================================================================
type LinkStateStrategy struct {
	logger *slog.Logger
}

func NewLinkStateStrategy(logger *slog.Logger) *LinkStateStrategy {
	return &LinkStateStrategy{logger: logger}
}

func (s *LinkStateStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 2

	// Fetch the latest interface operational statuses for this device
	// Corrected placement of DISTINCT ON
	query := `
        SELECT DISTINCT ON (interface_id) 
               interface_id, if_descr, if_oper_status 
        FROM interface_performance_metrics ipm
        JOIN interfaces i ON ipm.interface_id = i.id
        WHERE i.device_id = $1 AND ipm.time >= NOW() - INTERVAL '1 minute'
        ORDER BY interface_id, ipm.time DESC;`

	rows, err := database.Pool.Query(ctx, query, deviceID)
	if err != nil {
		s.logger.Error("LinkStateStrategy: failed to fetch interface metrics", "device_id", deviceID, "err", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var interfaceID int
		var ifDescr string
		var ifOperStatus int
		if err := rows.Scan(&interfaceID, &ifDescr, &ifOperStatus); err != nil {
			continue
		}

		// 2 = Down (Standard RFC1213/IF-MIB operational status definition)
		if ifOperStatus == 2 {
			// Read-Before-Write state validation to prevent duplicate alarm logging
			var placeholder int
			checkQuery := `SELECT 1 FROM network_events WHERE device_id = $1 AND alarm_id = $2 AND resolved = false AND message LIKE $3 LIMIT 1;`
			matchMsg := "%Interface " + ifDescr + "%"

			err = database.Pool.QueryRow(ctx, checkQuery, deviceID, alarmID, matchMsg).Scan(&placeholder)
			if errors.Is(err, pgx.ErrNoRows) {
				s.logger.Warn("LinkStateStrategy: Interface link down detected!", "device_id", deviceID, "interface", ifDescr)

				insertQuery := `
					INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, details, resolved) 
					VALUES (NOW(), $1, $2, 'link_down', 'WARNING', $3, false);`
				msg := "Interface " + ifDescr + " (ID: " + string(rune(interfaceID)) + ") has changed state to DOWN."
				if _, execErr := database.Pool.Exec(ctx, insertQuery, deviceID, alarmID, msg); execErr != nil {
					s.logger.Error("LinkStateStrategy: failed to insert event", "device_id", deviceID, "err", execErr)
				}
			}
		}
	}
	return nil
}

// =========================================================================
// STRATEGY 3: AdminOperMismatchStrategy (Alarm 4)
// Flags configuration anomalies: Admin state is UP (1) but Oper state is DOWN (2)
// =========================================================================
type AdminOperMismatchStrategy struct {
	logger *slog.Logger
}

func NewAdminOperMismatchStrategy(logger *slog.Logger) *AdminOperMismatchStrategy {
	return &AdminOperMismatchStrategy{logger: logger}
}

func (s *AdminOperMismatchStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 4

	// Corrected placement of DISTINCT ON
	query := `
        SELECT DISTINCT ON (interface_id) 
               interface_id, if_descr 
        FROM interface_performance_metrics ipm
        JOIN interfaces i ON ipm.interface_id = i.id
        WHERE i.device_id = $1 AND ipm.time >= NOW() - INTERVAL '1 minute'
        AND ipm.if_admin_status = 1 AND ipm.if_oper_status = 2
        ORDER BY interface_id, ipm.time DESC;`

	rows, err := database.Pool.Query(ctx, query, deviceID)
	if err != nil {
		s.logger.Error("AdminOperMismatchStrategy: query failed", "device_id", deviceID, "err", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var interfaceID int
		var ifDescr string
		if err := rows.Scan(&interfaceID, &ifDescr); err != nil {
			continue
		}

		var placeholder int
		checkQuery := `SELECT 1 FROM network_events WHERE device_id = $1 AND alarm_id = $2 AND resolved = false AND message LIKE $3 LIMIT 1;`
		matchMsg := "%" + ifDescr + "%"

		err = database.Pool.QueryRow(ctx, checkQuery, deviceID, alarmID, matchMsg).Scan(&placeholder)
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("AdminOperMismatchStrategy: Configuration mismatch state discovered!", "device_id", deviceID, "interface", ifDescr)

			insertQuery := `
				INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, details, resolved) 
				VALUES (NOW(), $1, $2, 'admin_oper_mismatch', 'WARNING', $3, false);`
			msg := "Interface Mismatch: " + ifDescr + " is administratively enabled (UP) but operationally DOWN."
			if _, execErr := database.Pool.Exec(ctx, insertQuery, deviceID, alarmID, msg); execErr != nil {
				s.logger.Error("AdminOperMismatchStrategy: failed to insert event", "device_id", deviceID, "err", execErr)
			}
		}
	}
	return nil
}

// =========================================================================
// STRATEGY 4: HighCPUUtilizationStrategy (Alarm 3)
// Checks if continuous resource utilization exceeds an enterprise ceiling threshold
// =========================================================================
type HighCPUUtilizationStrategy struct {
	logger       *slog.Logger
	thresholdPct float64
}

func NewHighCPUUtilizationStrategy(logger *slog.Logger, threshold float64) *HighCPUUtilizationStrategy {
	return &HighCPUUtilizationStrategy{
		logger:       logger,
		thresholdPct: threshold,
	}
}

func (s *HighCPUUtilizationStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 3

	// Aggregate values using an average threshold across our rolling evaluation window
	query := `
		SELECT COALESCE(AVG(cpu_utilization), 0) 
		FROM device_health_metrics 
		WHERE device_id = $1 AND time >= NOW() - INTERVAL '2 minutes';`

	var avgCPU float64
	err := database.Pool.QueryRow(ctx, query, deviceID).Scan(&avgCPU)
	if err != nil {
		s.logger.Error("HighCPUUtilizationStrategy: evaluation calculations failed", "device_id", deviceID, "err", err)
		return err
	}

	if avgCPU > s.thresholdPct {
		var placeholder int
		checkQuery := `SELECT 1 FROM network_events WHERE device_id = $1 AND alarm_id = $2 AND resolved = false LIMIT 1;`

		err = database.Pool.QueryRow(ctx, checkQuery, deviceID, alarmID).Scan(&placeholder)
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Error("HighCPUUtilizationStrategy: Resource exhaustion threshold crossed!", "device_id", deviceID, "avg_cpu", avgCPU)

			insertQuery := `
				INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, details, resolved) 
				VALUES (NOW(), $1, $2, 'high_cpu', 'CRITICAL', 'High compute usage warning: average CPU utilization is sustained above threshold bounds.', false);`
			if _, execErr := database.Pool.Exec(ctx, insertQuery, deviceID, alarmID); execErr != nil {
				s.logger.Error("HighCPUUtilizationStrategy: failed to insert event", "device_id", deviceID, "err", execErr)
			}
		}
	}
	return nil
}
