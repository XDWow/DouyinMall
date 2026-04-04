package domain

import (
	"context"
	"time"
)

type DelayQueue interface {
	Enqueue(ctx context.Context, orderID int64, executeAt time.Time) error
	DrainDue(ctx context.Context, now time.Time) ([]int64, error)
}


