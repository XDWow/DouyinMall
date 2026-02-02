package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisInventoryCache struct {
	cmd redis.Cmdable
}

func NewRedisInventoryCache(cmd redis.Cmdable) InventoryCache {
	return &redisInventoryCache{cmd}
}

func (c *redisInventoryCache) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return c.cmd.Eval(ctx, script, keys, args...).Result()
}

func (c *redisInventoryCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	return c.cmd.HMGet(ctx, key, fields...).Result()
}

func (c *redisInventoryCache) IncrBy(ctx context.Context, key string, delta int32) (int64, error) {
	return c.cmd.IncrBy(ctx, key, int64(delta)).Result()
}

func (c *redisInventoryCache) Get(ctx context.Context, key string) (string, error) {
	return c.cmd.Get(ctx, key).Result()
}

func (c *redisInventoryCache) Set(ctx context.Context, key string, value string, expiration time.Duration) (string, error) {
	return c.cmd.Set(ctx, key, value, expiration).Result()
}
