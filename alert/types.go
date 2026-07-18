package alert

import (
	"context"
	"time"

	"nms-middleware/db"
)

// AlarmStrategy is the interface every concrete alarm rule must satisfy.
type AlarmStrategy interface {
	Evaluate(ctx context.Context, database *db.DB, deviceID int) error
}

// Job is the unit of work dispatched to each evaluation worker goroutine.
type Job struct {
	DeviceID  int
	Timestamp time.Time // The exact wall-clock time this evaluation pass was triggered
}
