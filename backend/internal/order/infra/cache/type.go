package cache

import (
	"context"
	"time"
)

type OrderCache interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	MGet(ctx context.Context, keys ...string) ([]*string, error) // 批量获取，返回值对应keys顺序，nil表示不存在
	Del(ctx context.Context, keys ...string) error

	ZAdd(ctx context.Context, key string, members map[string]float64, ttl time.Duration) error
	// ZAddWithLimit: ZADD + 裁剪 + 设置TTL，Lua脚本保证原子性
	ZAddWithLimit(ctx context.Context, key string, members map[string]float64, limit int64, ttl time.Duration) error
	ZRange(ctx context.Context, key string, start, stop int64, reverse bool) ([]string, error)
	ZRem(ctx context.Context, key string, members ...string) error
}
