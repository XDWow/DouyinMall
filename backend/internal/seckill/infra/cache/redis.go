package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type pipelineProvider interface {
	Pipeline() redis.Pipeliner
}

// RedisCache 鍙彁渚涘熀纭€ Redis 鑳藉姏锛屼笉鍏冲績绉掓潃涓氬姟璇箟銆?
type RedisCache struct {
	cmd     redis.Cmdable
	txPiper pipelineProvider
}

func NewRedisCache(cmd redis.Cmdable) *RedisCache {
	rc := &RedisCache{cmd: cmd}
	if provider, ok := cmd.(pipelineProvider); ok {
		rc.txPiper = provider
	}
	return rc
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.cmd.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return c.cmd.Get(ctx, key).Result()
}

func (c *RedisCache) Del(ctx context.Context, keys ...string) error {
	return c.cmd.Del(ctx, keys...).Err()
}

func (c *RedisCache) IncrBy(ctx context.Context, key string, value int64) error {
	return c.cmd.IncrBy(ctx, key, value).Err()
}

func (c *RedisCache) EvalInt64(ctx context.Context, script string, keys []string, args ...interface{}) (int64, error) {
	return c.cmd.Eval(ctx, script, keys, args...).Int64()
}

func (c *RedisCache) Pipeline() (redis.Pipeliner, error) {
	if c.txPiper == nil {
		return nil, errors.New("redis client 涓嶆敮鎸?pipeline")
	}
	return c.txPiper.Pipeline(), nil
}


