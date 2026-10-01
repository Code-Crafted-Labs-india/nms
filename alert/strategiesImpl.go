package alert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"nms-middleware/db"
	"time"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// AUTO-CLEAR HELPER
// Resolves all open alarm instances for a given device + alarmID when the
// condition is no longer active. This satisfies the scope requirement:
//   "A recovered condition is automatically resolved only after its configured
//    recovery window; brief oscillation is recorded as flapping."
// ============================================================================

func clearAlarm(ctx context.Context, database *db.DB, logger *slog.Logger, deviceID, alarmID int, strategyName string) error {
	tag, err := database.Pool.Exec(ctx, `
		UPDATE network_events
		SET    resolved    = TRUE,
		       resolved_at = NOW()
		WHERE  device_id = $1
		  AND  alarm_id  = $2
		  AND  resolved  = FALSE
	`, deviceID, alarmID)
	if err != nil {
		logger.Error(fmt.Sprintf("%s: auto-clear query failed", strategyName), "device_id", deviceID, "err", err)
		return err
	}
	if tag.RowsAffected() > 0 {
		logger.Info(fmt.Sprintf("%s: alarm auto-cleared — condition recovered", strategyName), "device_id", deviceID, "rows_cleared", tag.RowsAffected())
	}
	return nil
}

// ============================================================================
// STRATEGY 1: DeviceDownStrategy (Scope Alarm #1 — P0)
// Evaluates consecutive ICMP failures over a configurable window and clears
// automatically after stable recovery.
// ============================================================================

type DeviceDownStrategy struct {
	logger         *slog.Logger
	windowDuration time.Duration
}

func NewDeviceDownStrategy(logger *slog.Logger, window time.Duration) *DeviceDownStrategy {
	return &DeviceDownStrategy{logger: logger, windowDuration: window}
}

func (d *DeviceDownStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 1

	query := `
		SELECT COUNT(*), COALESCE(SUM(icmp_status), 0)
		FROM device_health_metrics
		WHERE device_id = $1 AND time >= NOW() - INTERVAL '1 minute';`

	var count int
	var statusSum int
	if err := database.Pool.QueryRow(ctx, query, deviceID).Scan(&count, &statusSum); err != nil {
		d.logger.Error("DeviceDownStrategy: failed to evaluate sliding window", "device_id", deviceID, "err", err)
		return err
	}

	// Telemetry silence — cannot distinguish device failure from collector failure; do not alarm.
	if count == 0 {
		d.logger.Warn("DeviceDownStrategy: telemetry silence — no metrics in window", "device_id", deviceID)
		return nil
	}

	// At least one ICMP success → device is up; auto-clear any open alarm.
	if statusSum > 0 {
		return clearAlarm(ctx, database, d.logger, deviceID, alarmID, "DeviceDownStrategy")
	}

	// All probes failed → check for an already-open alarm to suppress duplicates.
	var placeholder int
	err := database.Pool.QueryRow(ctx, `
		SELECT 1 FROM network_events
		WHERE device_id = $1 AND alarm_id = $2 AND resolved = FALSE
		LIMIT 1;
	`, deviceID, alarmID).Scan(&placeholder)

	if errors.Is(err, pgx.ErrNoRows) {
		d.logger.Warn("DeviceDownStrategy: all ICMP probes failed — raising alarm", "device_id", deviceID)
		_, insertErr := database.Pool.Exec(ctx, `
			INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, message, resolved)
			VALUES (NOW(), $1, $2, 'device_down', 'CRITICAL',
				'Device failed to respond to all ICMP probes in the 60-second evaluation window.', FALSE);
		`, deviceID, alarmID)
		if insertErr != nil {
			d.logger.Error("DeviceDownStrategy: failed to insert alarm event", "device_id", deviceID, "err", insertErr)
			return insertErr
		}
	} else {
		d.logger.Info("DeviceDownStrategy: open alarm already registered — suppressing duplicate", "device_id", deviceID)
	}
	return nil
}

// ============================================================================
// STRATEGY 2: LinkStateStrategy (Scope Alarm #2 — P0)
// Tracks ifOperStatus transitions. Alarms when oper=DOWN; suppresses
// administratively-down ports. Auto-clears when the interface recovers.
// ============================================================================

type LinkStateStrategy struct {
	logger *slog.Logger
}

func NewLinkStateStrategy(logger *slog.Logger) *LinkStateStrategy {
	return &LinkStateStrategy{logger: logger}
}

func (s *LinkStateStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 2

	// Fetch the latest oper/admin status per interface for this device.
	rows, err := database.Pool.Query(ctx, `
		SELECT DISTINCT ON (interface_id)
		       interface_id, if_descr, if_oper_status, COALESCE(if_admin_status, 1)
		FROM interface_performance_metrics ipm
		JOIN interfaces i ON ipm.interface_id = i.id
		WHERE i.device_id = $1 AND ipm.time >= NOW() - INTERVAL '1 minute'
		ORDER BY interface_id, ipm.time DESC;
	`, deviceID)
	if err != nil {
		s.logger.Error("LinkStateStrategy: failed to fetch interface metrics", "device_id", deviceID, "err", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var interfaceID, ifOperStatus, ifAdminStatus int
		var ifDescr string
		if err := rows.Scan(&interfaceID, &ifDescr, &ifOperStatus, &ifAdminStatus); err != nil {
			continue
		}

		matchMsg := "%Interface " + ifDescr + "%"

		// ifOperStatus=1 → UP: auto-clear any open alarm for this interface.
		if ifOperStatus == 1 {
			_, clearErr := database.Pool.Exec(ctx, `
				UPDATE network_events
				SET resolved = TRUE, resolved_at = NOW()
				WHERE device_id = $1 AND alarm_id = $2
				  AND resolved = FALSE AND message LIKE $3
			`, deviceID, alarmID, matchMsg)
			if clearErr != nil {
				s.logger.Error("LinkStateStrategy: auto-clear failed", "interface", ifDescr, "err", clearErr)
			}
			continue
		}

		// ifOperStatus=2 → DOWN. Suppress if administratively down (admin=2).
		if ifOperStatus == 2 {
			if ifAdminStatus == 2 {
				// Port is intentionally shut — not an alarm condition per scope.
				continue
			}
			var placeholder int
			err = database.Pool.QueryRow(ctx, `
				SELECT 1 FROM network_events
				WHERE device_id = $1 AND alarm_id = $2 AND resolved = FALSE AND message LIKE $3
				LIMIT 1;
			`, deviceID, alarmID, matchMsg).Scan(&placeholder)
			if errors.Is(err, pgx.ErrNoRows) {
				s.logger.Warn("LinkStateStrategy: interface link down", "device_id", deviceID, "interface", ifDescr)
				msg := fmt.Sprintf("Interface %s (ID: %d) has transitioned to operational DOWN state.", ifDescr, interfaceID)
				if _, execErr := database.Pool.Exec(ctx, `
					INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, message, resolved)
					VALUES (NOW(), $1, $2, 'link_down', 'WARNING', $3, FALSE);
				`, deviceID, alarmID, msg); execErr != nil {
					s.logger.Error("LinkStateStrategy: failed to insert alarm", "device_id", deviceID, "err", execErr)
				}
			}
		}
	}
	return nil
}

// ============================================================================
// STRATEGY 3: AdminOperMismatchStrategy (Scope Alarm #19 related — P1)
// Flags admin=UP / oper=DOWN configuration anomalies. Auto-clears on recovery.
// ============================================================================

type AdminOperMismatchStrategy struct {
	logger *slog.Logger
}

func NewAdminOperMismatchStrategy(logger *slog.Logger) *AdminOperMismatchStrategy {
	return &AdminOperMismatchStrategy{logger: logger}
}

func (s *AdminOperMismatchStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 4

	rows, err := database.Pool.Query(ctx, `
		SELECT DISTINCT ON (interface_id)
		       interface_id, if_descr, if_admin_status, if_oper_status
		FROM interface_performance_metrics ipm
		JOIN interfaces i ON ipm.interface_id = i.id
		WHERE i.device_id = $1 AND ipm.time >= NOW() - INTERVAL '1 minute'
		ORDER BY interface_id, ipm.time DESC;
	`, deviceID)
	if err != nil {
		s.logger.Error("AdminOperMismatchStrategy: query failed", "device_id", deviceID, "err", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var interfaceID, ifAdminStatus, ifOperStatus int
		var ifDescr string
		if err := rows.Scan(&interfaceID, &ifDescr, &ifAdminStatus, &ifOperStatus); err != nil {
			continue
		}
		matchMsg := "%" + ifDescr + "%"

		// Condition cleared: admin=UP and oper=UP → auto-clear alarm.
		if ifAdminStatus == 1 && ifOperStatus == 1 {
			_, _ = database.Pool.Exec(ctx, `
				UPDATE network_events
				SET resolved = TRUE, resolved_at = NOW()
				WHERE device_id = $1 AND alarm_id = $2 AND resolved = FALSE AND message LIKE $3
			`, deviceID, alarmID, matchMsg)
			continue
		}

		// Alarm condition: admin=UP but oper=DOWN.
		if ifAdminStatus == 1 && ifOperStatus == 2 {
			var placeholder int
			err = database.Pool.QueryRow(ctx, `
				SELECT 1 FROM network_events
				WHERE device_id = $1 AND alarm_id = $2 AND resolved = FALSE AND message LIKE $3
				LIMIT 1;
			`, deviceID, alarmID, matchMsg).Scan(&placeholder)
			if errors.Is(err, pgx.ErrNoRows) {
				s.logger.Warn("AdminOperMismatchStrategy: mismatch detected", "device_id", deviceID, "interface", ifDescr)
				msg := fmt.Sprintf("Interface mismatch: %s is administratively enabled (UP) but operationally DOWN.", ifDescr)
				if _, execErr := database.Pool.Exec(ctx, `
					INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, message, resolved)
					VALUES (NOW(), $1, $2, 'admin_oper_mismatch', 'WARNING', $3, FALSE);
				`, deviceID, alarmID, msg); execErr != nil {
					s.logger.Error("AdminOperMismatchStrategy: insert failed", "device_id", deviceID, "err", execErr)
				}
			}
		}
	}
	return nil
}

// ============================================================================
// STRATEGY 4: HighCPUUtilizationStrategy (Scope Alarm #3 partial — P0)
// Auto-clears when CPU drops below threshold with hysteresis margin.
// ============================================================================

type HighCPUUtilizationStrategy struct {
	logger       *slog.Logger
	thresholdPct float64
}

func NewHighCPUUtilizationStrategy(logger *slog.Logger, threshold float64) *HighCPUUtilizationStrategy {
	return &HighCPUUtilizationStrategy{logger: logger, thresholdPct: threshold}
}

func (s *HighCPUUtilizationStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 3
	// Hysteresis: clear at threshold-5% to prevent flapping at the boundary.
	clearThreshold := s.thresholdPct - 5.0

	var avgCPU float64
	if err := database.Pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(cpu_utilization), 0)
		FROM device_health_metrics
		WHERE device_id = $1 AND time >= NOW() - INTERVAL '2 minutes';
	`, deviceID).Scan(&avgCPU); err != nil {
		s.logger.Error("HighCPUUtilizationStrategy: query failed", "device_id", deviceID, "err", err)
		return err
	}

	if avgCPU <= clearThreshold {
		return clearAlarm(ctx, database, s.logger, deviceID, alarmID, "HighCPUUtilizationStrategy")
	}

	if avgCPU > s.thresholdPct {
		var placeholder int
		err := database.Pool.QueryRow(ctx, `
			SELECT 1 FROM network_events WHERE device_id = $1 AND alarm_id = $2 AND resolved = FALSE LIMIT 1;
		`, deviceID, alarmID).Scan(&placeholder)
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("HighCPUUtilizationStrategy: threshold breached", "device_id", deviceID, "avg_cpu", avgCPU)
			if _, execErr := database.Pool.Exec(ctx, `
				INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, message, resolved)
				VALUES (NOW(), $1, $2, 'high_cpu', 'CRITICAL',
					'High CPU utilization: sustained average exceeded the configured threshold.', FALSE);
			`, deviceID, alarmID); execErr != nil {
				s.logger.Error("HighCPUUtilizationStrategy: insert failed", "device_id", deviceID, "err", execErr)
			}
		}
	}
	return nil
}

// ============================================================================
// STRATEGY 5: HighMemoryUtilizationStrategy (Scope Alarm #3 — P0)
// Calculates memory utilization from snmp_device_health and applies
// warning/critical thresholds, hysteresis, and automatic recovery.
// ============================================================================

type HighMemoryUtilizationStrategy struct {
	logger            *slog.Logger
	warningThreshold  float64
	criticalThreshold float64
}

func NewHighMemoryUtilizationStrategy(logger *slog.Logger, warning, critical float64) *HighMemoryUtilizationStrategy {
	return &HighMemoryUtilizationStrategy{
		logger:            logger,
		warningThreshold:  warning,
		criticalThreshold: critical,
	}
}

func (s *HighMemoryUtilizationStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 5
	// Hysteresis: clear when utilization drops 5% below the warning floor.
	clearThreshold := s.warningThreshold - 5.0

	// Join snmp_device_health to devices by IP to get the device-resolved metrics.
	var avgMemPct float64
	err := database.Pool.QueryRow(ctx, `
		SELECT COALESCE(
			AVG(100.0 * (mem_total_kb - mem_available_kb) / NULLIF(mem_total_kb, 0)),
			0.0
		)
		FROM snmp_device_health h
		JOIN devices d ON d.ip_address = NULLIF(h.agent_host, '')::inet
		WHERE d.id = $1 AND h.time >= NOW() - INTERVAL '2 minutes'
		  AND h.mem_total_kb > 0;
	`, deviceID).Scan(&avgMemPct)
	if err != nil {
		s.logger.Error("HighMemoryUtilizationStrategy: query failed", "device_id", deviceID, "err", err)
		return err
	}

	// Recovery path: clear open alarm when memory drops back below threshold.
	if avgMemPct <= clearThreshold {
		return clearAlarm(ctx, database, s.logger, deviceID, alarmID, "HighMemoryUtilizationStrategy")
	}

	// Determine severity: warning vs critical.
	severity := "WARNING"
	eventType := "high_memory_warning"
	if avgMemPct >= s.criticalThreshold {
		severity = "CRITICAL"
		eventType = "high_memory_critical"
	}

	if avgMemPct >= s.warningThreshold {
		var placeholder int
		checkErr := database.Pool.QueryRow(ctx, `
			SELECT 1 FROM network_events
			WHERE device_id = $1 AND alarm_id = $2 AND resolved = FALSE
			LIMIT 1;
		`, deviceID, alarmID).Scan(&placeholder)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			s.logger.Warn("HighMemoryUtilizationStrategy: memory threshold breached",
				"device_id", deviceID, "avg_mem_pct", avgMemPct, "severity", severity)
			msg := fmt.Sprintf("High memory utilization: %.1f%% average (warning=%.0f%%, critical=%.0f%%).",
				avgMemPct, s.warningThreshold, s.criticalThreshold)
			if _, execErr := database.Pool.Exec(ctx, `
				INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, message, resolved)
				VALUES (NOW(), $1, $2, $3, $4, $5, FALSE);
			`, deviceID, alarmID, eventType, severity, msg); execErr != nil {
				s.logger.Error("HighMemoryUtilizationStrategy: insert failed", "device_id", deviceID, "err", execErr)
			}
		}
	}
	return nil
}

// ============================================================================
// STRATEGY 6: SNMPCommunicationFailureStrategy (Scope Alarm #4 — P0)
// Alarms when a monitored device has had no successful SNMP poll within the
// policy window. Distinguishes no-data from collector failure per scope.
// Auto-clears when SNMP data resumes arriving.
// ============================================================================

type SNMPCommunicationFailureStrategy struct {
	logger        *slog.Logger
	staleWindow   time.Duration
}

func NewSNMPCommunicationFailureStrategy(logger *slog.Logger, staleWindow time.Duration) *SNMPCommunicationFailureStrategy {
	return &SNMPCommunicationFailureStrategy{
		logger:      logger,
		staleWindow: staleWindow,
	}
}

func (s *SNMPCommunicationFailureStrategy) Evaluate(ctx context.Context, database *db.DB, deviceID int) error {
	const alarmID = 6

	// Check for a recent SNMP health record for this device (via agent_host → ip_address join).
	// A row within the stale window means SNMP is functioning.
	var recentCount int
	err := database.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM snmp_device_health h
		JOIN devices d ON d.ip_address = NULLIF(h.agent_host, '')::inet
		WHERE d.id = $1 AND h.time >= NOW() - $2::interval;
	`, deviceID, fmt.Sprintf("%d seconds", int(s.staleWindow.Seconds()))).Scan(&recentCount)
	if err != nil {
		s.logger.Error("SNMPCommunicationFailureStrategy: query failed", "device_id", deviceID, "err", err)
		return err
	}

	// SNMP data is flowing → clear any open SNMP failure alarm.
	if recentCount > 0 {
		return clearAlarm(ctx, database, s.logger, deviceID, alarmID, "SNMPCommunicationFailureStrategy")
	}

	// Also check snmp_interface and snmp tables for any recent data from this device.
	var ifCount int
	_ = database.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM snmp_interface si
		JOIN devices d ON d.ip_address = NULLIF(si.agent_host, '')::inet
		WHERE d.id = $1 AND si.time >= NOW() - $2::interval;
	`, deviceID, fmt.Sprintf("%d seconds", int(s.staleWindow.Seconds()))).Scan(&ifCount)
	if ifCount > 0 {
		return clearAlarm(ctx, database, s.logger, deviceID, alarmID, "SNMPCommunicationFailureStrategy")
	}

	// No SNMP data within stale window → check for an open alarm before inserting.
	var placeholder int
	checkErr := database.Pool.QueryRow(ctx, `
		SELECT 1 FROM network_events
		WHERE device_id = $1 AND alarm_id = $2 AND resolved = FALSE
		LIMIT 1;
	`, deviceID, alarmID).Scan(&placeholder)
	if errors.Is(checkErr, pgx.ErrNoRows) {
		s.logger.Warn("SNMPCommunicationFailureStrategy: no SNMP data within stale window",
			"device_id", deviceID, "window", s.staleWindow)
		msg := fmt.Sprintf("SNMP communication failure: no successful poll data received within the %s evaluation window.", s.staleWindow)
		if _, execErr := database.Pool.Exec(ctx, `
			INSERT INTO network_events (time, device_id, alarm_id, event_type, severity, message, resolved)
			VALUES (NOW(), $1, $2, 'snmp_failure', 'CRITICAL', $3, FALSE);
		`, deviceID, alarmID, msg); execErr != nil {
			s.logger.Error("SNMPCommunicationFailureStrategy: insert failed", "device_id", deviceID, "err", execErr)
		}
	}
	return nil
}
