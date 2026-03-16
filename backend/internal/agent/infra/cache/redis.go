package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type AgentCache interface {
	Get(ctx context.Context, key string) (string, error)
	MGet(ctx context.Context, keys ...string) ([]string, error) // 批量查询，缺失的 key 返回空字符串
	Set(ctx context.Context, key string, val string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	RPush(ctx context.Context, key string, vals ...string) error
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LTrim(ctx context.Context, key string, start, stop int64) error
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type agentRedisCache struct {
	client redis.Cmdable
}

// NewAgentRedis 创建通用 Redis 访问层
func NewAgentRedis(client redis.Cmdable) AgentCache {
	return &agentRedisCache{client: client}
}

func (a *agentRedisCache) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}

func (a *agentRedisCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	vals, err := a.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	res := make([]string, len(vals))
	for i, v := range vals {
		if s, ok := v.(string); ok {
			res[i] = s
		} // nil（key 不存在）保持空字符串
	}
	return res, nil
}

func (a *agentRedisCache) Set(ctx context.Context, key string, val string, ttl time.Duration) error {
	return a.client.Set(ctx, key, val, ttl).Err()
}

func (a *agentRedisCache) Del(ctx context.Context, keys ...string) error {
	return a.client.Del(ctx, keys...).Err()
}

func (a *agentRedisCache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return a.client.Eval(ctx, script, keys, args...).Result()
}

func (a *agentRedisCache) RPush(ctx context.Context, key string, vals ...string) error {
	args := make([]any, len(vals))
	for i, v := range vals {
		args[i] = v
	}
	return a.client.RPush(ctx, key, args...).Err()
}

func (a *agentRedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return a.client.LRange(ctx, key, start, stop).Result()
}

func (a *agentRedisCache) LTrim(ctx context.Context, key string, start, stop int64) error {
	return a.client.LTrim(ctx, key, start, stop).Err()
}

func (a *agentRedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.client.Expire(ctx, key, ttl).Err()
}
