package activityconsumer

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrActivityBusy   = errors.New("activity busy")
	ErrMailboxFull    = errors.New("activity mailbox full")
	ErrConsumerClosed = errors.New("consumer closed")
	ErrInvalidConfig  = errors.New("invalid config")
	ErrNilProcessFunc = errors.New("nil process func")
)

// Message is the generic unit consumed by the activity-aware consumer.
type Message struct {
	ActivityID int64
	RequestID  string
	Payload    any
}

// ProcessFunc contains the real business logic for a message.
type ProcessFunc func(ctx context.Context, msg Message) error

// ErrorHook lets upper layers observe asynchronous failures.
type ErrorHook func(ctx context.Context, msg Message, err error)

// Config tunes the single-instance consumer.
type Config struct {
	PoolSize               int
	PerActivityLimit       int
	PerActivityMailboxSize int
	ActivityTTL            time.Duration
	GCInterval             time.Duration
	ShardCount             int

	OnProcessError ErrorHook
	OnSubmitError  ErrorHook
}

func (c Config) validate() error {
	switch {
	case c.PoolSize <= 0:
		return fmt.Errorf("%w: pool size must be > 0", ErrInvalidConfig)
	case c.PerActivityLimit <= 0:
		return fmt.Errorf("%w: per-activity limit must be > 0", ErrInvalidConfig)
	case c.PerActivityMailboxSize < 0:
		return fmt.Errorf("%w: per-activity mailbox size must be >= 0", ErrInvalidConfig)
	case c.ActivityTTL <= 0:
		return fmt.Errorf("%w: activity TTL must be > 0", ErrInvalidConfig)
	case c.GCInterval <= 0:
		return fmt.Errorf("%w: GC interval must be > 0", ErrInvalidConfig)
	case c.ShardCount <= 0:
		return fmt.Errorf("%w: shard count must be > 0", ErrInvalidConfig)
	default:
		return nil
	}
}
