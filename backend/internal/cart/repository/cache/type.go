package cache

import "context"

type CartCache interface {
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HSet(ctx context.Context, key, field string, value int64) error
	HSetBatch(ctx context.Context, key string, fieldValues map[string]int64) error
	HIncrBy(ctx context.Context, key, field string, increment int64) (int64, error)
	HIncrByBatch(ctx context.Context, key string, fieldIncrements map[string]int64) (map[string]int64, error)
	HDel(ctx context.Context, key string, fields ...string) error
	Del(ctx context.Context, keys ...string) error
	DecrementIfGreaterThanOne(ctx context.Context, key, field string) (int64, error)
}
