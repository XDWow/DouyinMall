package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisOrderCache struct {
	client redis.Cmdable
}

func NewRedisOrderCache(client redis.Cmdable) OrderCache {
	return &redisOrderCache{client: client}
}

func (c *redisOrderCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *redisOrderCache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *redisOrderCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *redisOrderCache) ZAdd(ctx context.Context, key string, members map[string]float64, ttl time.Duration) error {
	if len(members) == 0 {
		return nil
	}

	zs := make([]redis.Z, 0, len(members))
	for member, score := range members {
		zs = append(zs, redis.Z{Score: score, Member: member})
	}

	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, key, zs...)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redisOrderCache) ZRange(ctx context.Context, key string, start, stop int64, reverse bool) ([]string, error) {
	if reverse {
		return c.client.ZRevRange(ctx, key, start, stop).Result()
	}
	return c.client.ZRange(ctx, key, start, stop).Result()
}

func (c *redisOrderCache) ZRem(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	interfaces := make([]interface{}, len(members))
	for i, m := range members {
		interfaces[i] = m
	}
	return c.client.ZRem(ctx, key, interfaces...).Err()
}
