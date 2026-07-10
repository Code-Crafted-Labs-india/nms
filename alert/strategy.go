package alert

import (
	"context"
	"nms-middleware/db"
	"time"
)

type AlarmStrategy interface {
	Evaluate(ctx context.Context, database *db.DB, deviceID int) error
}

type Job struct {
	DeviceID  int
	Timestamp time.Time // The exact wall-clock time this evaluation pass was triggered
}

// Concrete implementation structures will live in your other files,
// and they will be registered at startup like this:
//
// func NewDeviceDownStrategy() AlarmStrategy { ... }
// func NewLinkStateStrategy() AlarmStrategy { ... }
