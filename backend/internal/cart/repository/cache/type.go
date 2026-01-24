package cache

import (
	"context"
)

type CartCache interface {
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HSet(ctx context.Context, key, field string, value int64) error
	HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error)
	HDel(ctx context.Context, key, field string) error
	Del(ctx context.Context, key string) error
	DecrementIfGreaterThanOne(ctx context.Context, key, field string) (int64, error)
}
