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

func (c *RedisCache) HSetBatch(ctx context.Context, key string, fieldValues map[string]int64) error {
	if len(fieldValues) == 0 {
		return nil
	}
	args := make([]interface{}, 0, len(fieldValues)*2)
	for f, v := range fieldValues {
		args = append(args, f, v)
	}
	if err := c.client.HSet(ctx, key, args...).Err(); err != nil {
		return err
	}
	return c.client.Expire(ctx, key, c.ttl).Err()
}

func (c *RedisCache) HIncrBy(ctx context.Context, key, field string, increment int64) (int64, error) {
	result, err := c.client.HIncrBy(ctx, key, field, increment).Result()
	if err != nil {
		return 0, err
	}
	return result, c.client.Expire(ctx, key, c.ttl).Err()
}

func (c *RedisCache) HIncrByBatch(ctx context.Context, key string, fieldIncrements map[string]int64) (map[string]int64, error) {
	if len(fieldIncrements) == 0 {
		return nil, nil
	}
	pipe := c.client.Pipeline()
	cmds := make(map[string]*redis.IntCmd, len(fieldIncrements))
	for field, increment := range fieldIncrements {
		cmds[field] = pipe.HIncrBy(ctx, key, field, increment)
	}
	pipe.Expire(ctx, key, c.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(cmds))
	for field, cmd := range cmds {
		result[field] = cmd.Val()
	}
	return result, nil
}

func (c *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
}

func (c *RedisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *RedisCache) DecrementIfGreaterThanOne(ctx context.Context, key, field string) (int64, error) {
	result, err := c.client.Eval(ctx, decrementScript, []string{key}, field, int64(c.ttl.Seconds())).Result()
	if err != nil {
		return 0, err
	}

	newQty, ok := result.(int64)
	if !ok || newQty < 0 {
		return 0, fmt.Errorf("cart item quantity cannot be decremented")
	}

	return newQty, nil
}
