package ping

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

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
	icmpStatus := 0
	packetLoss := 100.0
	rtt := 0.0

	// 1. Strip CIDR mask if present from IPAddress
	ipStr := strings.Split(t.IPAddress, "/")[0]

	// 2. Open ICMP connection on raw socket
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		pe.logger.Warn("failed to listen for icmp on raw socket", "ip", ipStr, "error", err)
		pe.savePingResult(ctx, timestamp, t.DeviceID, icmpStatus, rtt, packetLoss)
		return
	}
	defer c.Close()

	// 3. Construct ICMP Echo message
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("HELLO-NMS"),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		pe.logger.Warn("failed to marshal icmp message", "ip", ipStr, "error", err)
		pe.savePingResult(ctx, timestamp, t.DeviceID, icmpStatus, rtt, packetLoss)
		return
	}

	dst, err := net.ResolveIPAddr("ip4", ipStr)
	if err != nil {
		pe.logger.Warn("failed to resolve ip", "ip", ipStr, "error", err)
		pe.savePingResult(ctx, timestamp, t.DeviceID, icmpStatus, rtt, packetLoss)
		return
	}

	// 4. Set a tight read/write deadline
	c.SetDeadline(time.Now().Add(2 * time.Second))

	start := time.Now()
	if _, err := c.WriteTo(wb, dst); err != nil {
		pe.logger.Warn("failed to write icmp message", "ip", ipStr, "error", err)
		pe.savePingResult(ctx, timestamp, t.DeviceID, icmpStatus, rtt, packetLoss)
		return
	}

	rb := make([]byte, 1500)
	n, _, err := c.ReadFrom(rb)
	if err != nil {
		pe.logger.Warn("device path unreachable or target down", "ip", ipStr)
	} else {
		rm, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), rb[:n])
		if err == nil && rm.Type == ipv4.ICMPTypeEchoReply {
			icmpStatus = 1
			packetLoss = 0.0
			rtt = float64(time.Since(start).Nanoseconds()) / 1e6 // fractional ms
			pe.logger.Info("device path responsive, sweep transaction verified", "ip", ipStr, "rtt_ms", rtt)
		} else {
			pe.logger.Warn("invalid icmp reply received", "ip", ipStr)
		}
	}

	// 5. Force commit directly down to storage layer
	pe.savePingResult(ctx, timestamp, t.DeviceID, icmpStatus, rtt, packetLoss)
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
