package alert

import (
	"context"
	"log/slog"
	"nms-middleware/db"
	"time"
)

type EvaluateEngine struct {
	db         *db.DB
	logger     *slog.Logger
	strategies []AlarmStrategy
}

func NewEvaluateEngine(db *db.DB, logger *slog.Logger, strategies []AlarmStrategy) *EvaluateEngine {
	return &EvaluateEngine{
		db:         db,
		logger:     logger,
		strategies: strategies,
	}
}

// StartWorkerPool spins up your fixed consumer threads
func (eve *EvaluateEngine) StartWorkerPool(ctx context.Context, workerCount int, interval time.Duration) {
	// 1. TODO: Create your buffered jobs channel
	jobChannel := make(chan Job, 100)
	// 2. TODO: Spawn your fixed number of worker goroutines
	for i := range workerCount {
		go eve.worker(ctx, i, jobChannel)
	}
	// 3. TODO: Run a ticker loop that feeds device IDs into the channel
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		defer close(jobChannel) // ! safe tear down signal for worker ranges

		for {
			select {
			case <-ctx.Done():
				eve.logger.Warn("Shoutdown signal intercepted. Terminating evaluation producer loop.")
				return

			case <-ticker.C:
				eve.logger.Info("Interval tick hit. Orchestrating device evaluation passes.")

				// TODO: Replace this seed payload with your real relational database query:
				devices, err := eve.db.Pool.Query(ctx, "SELECT * FROM devices")
				if err != nil {
					eve.logger.Warn("Some error occured while fetching data from devices", err)
				}
				eve.logger.Info("fetched devices response", (devices))

				// For now, we simulate writing a single node target event down the data highway
				select {
				case jobChannel <- Job{DeviceID: 123, Timestamp: time.Now()}:
				default:
					eve.logger.Error("Job channel buffer is full! Evaluation events are dropping. Scale your worker pool configuration.")
				}
			}
		}

	}()

	eve.logger.Info("Starting concurrent native evaluation loop", "interval", interval.String())

	jobChannel <- Job{DeviceID: 123, Timestamp: time.Now()}

}

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
