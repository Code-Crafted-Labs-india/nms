package ping

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"nms-middleware/db"

	probing "github.com/prometheus-community/pro-bing"
)

type PingEngine struct {
	db     *db.DB
	logger *slog.Logger
}

func NewPingEngine(db *db.DB, logger *slog.Logger) *PingEngine {
	return &PingEngine{db: db, logger: logger}
}

// Target encapsulates target device criteria
type Target struct {
	DeviceID  int
	IPAddress string
}

// StartSweeper initializes a continuous background interval loop
func (pe *PingEngine) StartSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	pe.logger.Info("Starting concurrent ICMP ping sweeper loop", "interval", interval.String())

	for {
		select {
		case <-ctx.Done():
			pe.logger.Info("Stopping ICMP ping sweeper loop gracefully")
			return
		case <-ticker.C:
			targets, err := pe.fetchMonitoredDevices(ctx)
			if err != nil {
				pe.logger.Error("failed to retrieve active monitoring targets from inventory", "error", err)
				continue
			}

			if len(targets) == 0 {
				continue
			}

			pe.executeParallelSweep(ctx, targets)
		}
	}
}

// Fetch active devices from the relational directory
func (pe *PingEngine) fetchMonitoredDevices(ctx context.Context) ([]Target, error) {
	rows, err := pe.db.Pool.Query(ctx, "SELECT id, ip_address::text FROM devices WHERE is_monitored = true")
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

// Spin a controlled concurrent sync group over execution targets
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
	pinger, err := probing.NewPinger(t.IPAddress)
	if err != nil {
		pe.logger.Debug("failed to parse target IP address for icmp raw engine", "ip", t.IPAddress, "error", err)
		return
	}

	// Windows/macOS require privileged mode for raw ICMP unless explicitly allowed in shell
	pinger.SetPrivileged(true)
	pinger.Count = 3
	pinger.Timeout = 1 * time.Second

	err = pinger.Run() // Blocks until completes count parameters
	if err != nil {
		pe.logger.Warn("ping routine execution block failed", "ip", t.IPAddress, "error", err)
		pe.savePingResult(ctx, timestamp, t.DeviceID, 0, 0, 100.0) // Force 100% loss mapping
		return
	}

	stats := pinger.Statistics()
	icmpStatus := 1
	if stats.PacketsRecv == 0 {
		icmpStatus = 0 // Absolute device down trace marker
	}

	pe.savePingResult(ctx, timestamp, t.DeviceID, icmpStatus, stats.AvgRtt.Seconds()*1000, stats.PacketLoss)
}

func (pe *PingEngine) savePingResult(ctx context.Context, t time.Time, deviceID int, status int, rtt float64, loss float64) {
	query := `
		INSERT INTO device_health_metrics (time, device_id, icmp_status, icmp_rtt_ms, icmp_packet_loss)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := pe.db.Pool.Exec(ctx, query, t, deviceID, status, rtt, loss)
	if err != nil {
		pe.logger.Error("failed to write time-series ping health metric to hypertable", "device_id", deviceID, "error", err)
	}
}
