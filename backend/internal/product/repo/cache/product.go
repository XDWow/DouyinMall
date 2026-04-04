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
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error // 璁剧疆缂撳瓨骞舵寚瀹氳繃鏈熸椂闂?
	Delete(ctx context.Context, key string) error
	GetTTL(ctx context.Context, key string) (time.Duration, error) // 鑾峰彇鍓╀綑杩囨湡鏃堕棿

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

// BatchSetWithTTL 鎵归噺璁剧疆缂撳瓨锛屼娇鐢?Pipeline 鍑忓皯缃戠粶寰€杩?
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


