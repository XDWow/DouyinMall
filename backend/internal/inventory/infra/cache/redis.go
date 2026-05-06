package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type redisInventoryCache struct {
	cmd redis.Cmdable
}

func NewRedisInventoryCache(cmd redis.Cmdable) InventoryCache {
	return &redisInventoryCache{cmd: cmd}
}

func (c *redisInventoryCache) IncrBy(ctx context.Context, key string, delta int32) (int64, error) {
	return c.cmd.IncrBy(ctx, key, int64(delta)).Result()
}
