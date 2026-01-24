package cache

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed decrement.lua
var decrementScript string

type RedisCache struct {
	client redis.Cmdable
	ttl    time.Duration
}

func NewRedisCache(client redis.Cmdable) CartCache {
	return &RedisCache{
		client: client,
		ttl:    7 * 24 * time.Hour,
	}
}

func (c *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

func (c *RedisCache) HSet(ctx context.Context, key, field string, value int64) error {
	err := c.client.HSet(ctx, key, field, value).Err()
	if err != nil {
		return err
	}
	return c.client.Expire(ctx, key, c.ttl).Err()
}

func (c *RedisCache) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	newVal, err := c.client.HIncrBy(ctx, key, field, incr).Result()
	if err != nil {
		return 0, err
	}
	c.client.Expire(ctx, key, c.ttl)
	return newVal, nil
}

func (c *RedisCache) HDel(ctx context.Context, key, field string) error {
	return c.client.HDel(ctx, key, field).Err()
}

func (c *RedisCache) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) DecrementIfGreaterThanOne(ctx context.Context, key, field string) (int64, error) {
	result, err := c.client.Eval(ctx, decrementScript, []string{key}, field, int64(c.ttl.Seconds())).Result()
	if err != nil {
		return 0, err
	}

	newQty, ok := result.(int64)
	if !ok || newQty < 0 {
		return 0, fmt.Errorf("商品数量不能再减少了")
	}

	return newQty, nil
}
