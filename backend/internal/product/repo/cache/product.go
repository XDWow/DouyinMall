package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type ProductCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value interface{}) error
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error // 设置缓存并指定过期时间
	Delete(ctx context.Context, key string) error
	GetTTL(ctx context.Context, key string) (time.Duration, error) // 获取剩余过期时间

	BatchSetWithTTL(ctx context.Context, items []CacheItem) error
}

type CacheItem struct {
	Key   string
	Value interface{}
	TTL   time.Duration
}

type RedisProductCache struct {
	cmd        redis.Cmdable
	expiration time.Duration
}

func NewRedisProductCache(cmd redis.Cmdable) ProductCache {
	return &RedisProductCache{
		cmd:        cmd,
		expiration: time.Minute * 15,
	}
}

func (c *RedisProductCache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.cmd.Get(ctx, key).Bytes()
}

func (c *RedisProductCache) Set(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.cmd.Set(ctx, key, data, c.expiration).Err()
}

func (c *RedisProductCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.cmd.Set(ctx, key, data, ttl).Err()
}

func (c *RedisProductCache) Delete(ctx context.Context, key string) error {
	return c.cmd.Del(ctx, key).Err()
}

func (c *RedisProductCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return c.cmd.TTL(ctx, key).Result()
}

// BatchSetWithTTL 批量设置缓存，使用 Pipeline 减少网络往返
func (c *RedisProductCache) BatchSetWithTTL(ctx context.Context, items []CacheItem) error {
	if len(items) == 0 {
		return nil
	}

	pipe := c.cmd.Pipeline()
	for _, item := range items {
		data, err := json.Marshal(item.Value)
		if err != nil {
			return err
		}
		pipe.Set(ctx, item.Key, data, item.TTL)
	}

	_, err := pipe.Exec(ctx)
	return err
}
