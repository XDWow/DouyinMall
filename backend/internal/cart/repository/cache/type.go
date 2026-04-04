package cache

import (
	"context"
)

type CartCache interface {
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HSet(ctx context.Context, key, field string, value int64) error
	HSetBatch(ctx context.Context, key string, fieldValues map[string]int64) error
	HIncrBy(ctx context.Context, key string, fields ...string) ([]int64, error) // 鍗曚釜鎴栨壒閲?HINCRBY +1锛孭ipeline 涓€娆″線杩?
	HDel(ctx context.Context, key string, fields ...string) error
	Del(ctx context.Context, key string) error
	DecrementIfGreaterThanOne(ctx context.Context, key, field string) (int64, error)
}


