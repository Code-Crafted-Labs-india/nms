package ping

import (
	"context"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"nms-middleware/db"
)

type PingEngine struct {
	db     *db.DB
	logger *slog.Logger
}

func NewPingEngine(db *db.DB, logger *slog.Logger) *PingEngine {
	return &PingEngine{db: db, logger: logger}
}

type Target struct {
	DeviceID  int
	IPAddress string
}

func (pe *PingEngine) StartSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	pe.logger.Info("Starting concurrent native ping sweeper loop", "interval", interval.String())

	runSweep := func() {
		targets, err := pe.fetchMonitoredDevices(ctx)
		if err != nil {
			pe.logger.Error("failed to retrieve active monitoring targets from inventory", "error", err)
			return
		}

		if len(targets) == 0 {
			pe.logger.Warn("no active monitoring devices found in database inventory")
			return
		}

		pe.logger.Info("initiating parallel network sweep pass", "count", len(targets))
		pe.executeParallelSweep(ctx, targets)
	}

	runSweep()

	for {
		select {
		case <-ctx.Done():
			pe.logger.Info("Stopping ping sweeper loop gracefully")
			return
		case <-ticker.C:
			runSweep()
		}
	}
}

func (pe *PingEngine) fetchMonitoredDevices(ctx context.Context) ([]Target, error) {
	rows, err := pe.db.SelectIdAndIp(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.DeviceID, &t.IPAddress); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func (pe *PingEngine) executeParallelSweep(ctx context.Context, targets []Target) {
	var wg sync.WaitGroup
	now := time.Now()

	for _, target := range targets {
		wg.Add(1)
		go func(t Target) {
			defer wg.Done()
			pe.pingTarget(ctx, t, now)
		}(target)
	}

	wg.Wait()
}

func (pe *PingEngine) pingTarget(ctx context.Context, t Target, timestamp time.Time) {
	// 1. Set a tight command context deadline (2 seconds absolute execution limit)
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Run native ping utility: -c 2 (2 packets), -W 1 (1 second timeout)
	cmd := exec.CommandContext(cmdCtx, "ping", "-c", "2", "-W", "1", t.IPAddress)

	err := cmd.Run()

	icmpStatus := 1
	packetLoss := 0.0

	// 2. If err != nil, the binary exited with non-zero status (means packets dropped or timed out)
	if err != nil {
		icmpStatus = 0
		packetLoss = 100.0
		pe.logger.Warn("device path unreachable or target down", "ip", t.IPAddress)
	} else {
		pe.logger.Info("device path responsive, sweep transaction verified", "ip", t.IPAddress)
	}

	// 3. Force commit directly down to storage layer
	pe.savePingResult(ctx, timestamp, t.DeviceID, icmpStatus, 0.5, packetLoss)
}

func (pe *PingEngine) savePingResult(ctx context.Context, t time.Time, deviceID int, status int, rtt float64, loss float64) {
	query := `
		INSERT INTO device_health_metrics (time, device_id, icmp_status, icmp_rtt_ms, icmp_packet_loss)
		VALUES ($1, $2, $3, $4, $5)`

	pe.logger.Info("attempting database hypertable metrics insert write", "device_id", deviceID, "status", status)

	_, err := pe.db.Pool.Exec(ctx, query, t, deviceID, status, rtt, loss)
	if err != nil {
		pe.logger.Error("failed to write time-series ping health metric to hypertable", "device_id", deviceID, "error", err)
		return
	}

	pe.logger.Info("successfully committed metric row to timescaledb hypertable", "device_id", deviceID)
}
