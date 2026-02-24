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

// HIncrBy 通过 Pipeline 对一个或多个 field 执行 HINCRBY +1，只需一次网络往返
func (c *RedisCache) HIncrBy(ctx context.Context, key string, fields ...string) ([]int64, error) {
	pipe := c.client.Pipeline()
	cmds := make([]*redis.IntCmd, len(fields))
	for i, field := range fields {
		cmds[i] = pipe.HIncrBy(ctx, key, field, 1)
	}
	pipe.Expire(ctx, key, c.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	result := make([]int64, len(fields))
	for i, cmd := range cmds {
		result[i] = cmd.Val()
	}
	return result, nil
}

// HSetBatch 通过单条 HSET 命令批量写入多个 field-value（Redis 3.0+ 支持可变参数）
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

func (c *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
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
