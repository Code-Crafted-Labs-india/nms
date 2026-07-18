package alert

import (
	"context"
	"log/slog"
	"time"

	"nms-middleware/db"
)

// EvaluateEngine orchestrates periodic strategy evaluation across all monitored devices.
type EvaluateEngine struct {
	db         *db.DB
	logger     *slog.Logger
	strategies []AlarmStrategy
}

// NewEvaluateEngine constructs an EvaluateEngine with the registered alarm strategy set.
func NewEvaluateEngine(db *db.DB, logger *slog.Logger, strategies []AlarmStrategy) *EvaluateEngine {
	return &EvaluateEngine{
		db:         db,
		logger:     logger,
		strategies: strategies,
	}
}

// worker consumes jobs from the channel and evaluates all strategies against each device.
func (eve *EvaluateEngine) worker(ctx context.Context, id int, jobs <-chan Job) {
	for job := range jobs {
		eve.logger.Info("worker executing evaluation", "worker_id", id, "device_id", job.DeviceID)

		for _, strategy := range eve.strategies {
			if err := strategy.Evaluate(ctx, eve.db, job.DeviceID); err != nil {
				eve.logger.Error("strategy evaluation failure", "device_id", job.DeviceID, "err", err)
			}
		}
	}
}

// StartWorkerPool spins up a fixed consumer thread pool and drives them with a ticker.
func (eve *EvaluateEngine) StartWorkerPool(ctx context.Context, workerCount int, interval time.Duration) {
	jobChannel := make(chan Job, 100)

	for i := range workerCount {
		go eve.worker(ctx, i, jobChannel)
	}

	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		defer close(jobChannel) // Safe teardown: drains workers cleanly

		eve.logger.Info("Starting concurrent native evaluation loop", "interval", interval.String())

		for {
			select {
			case <-ctx.Done():
				eve.logger.Warn("Shoutdown signal intercepted. Terminating evaluation producer loop.")
				return

			case <-ticker.C:
				eve.logger.Info("Interval tick hit. Orchestrating device evaluation passes.")

				rows, err := eve.db.Pool.Query(ctx, "SELECT id FROM devices WHERE is_monitored = true")
				if err != nil {
					eve.logger.Error("Failed to fetch monitored devices from directory", "err", err)
					continue
				}

				func() {
					defer rows.Close()

					now := time.Now()
					for rows.Next() {
						var deviceID int
						if err := rows.Scan(&deviceID); err != nil {
							eve.logger.Error("Failed to scan device ID row", "err", err)
							continue
						}

						select {
						case jobChannel <- Job{DeviceID: deviceID, Timestamp: now}:
						default:
							eve.logger.Error("Job queue saturated! Drops detected.", "device_id", deviceID)
						}
					}
				}()
			}
		}
	}()
}
