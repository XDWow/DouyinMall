package cache

import (
	"context"
	"time"
)

type OrderCache interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error

	ZAdd(ctx context.Context, key string, members map[string]float64, ttl time.Duration) error
	ZRange(ctx context.Context, key string, start, stop int64, reverse bool) ([]string, error)
	ZRem(ctx context.Context, key string, members ...string) error
}
