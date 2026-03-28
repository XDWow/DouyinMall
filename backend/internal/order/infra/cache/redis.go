package cache

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/zadd_with_limit.lua
var zaddWithLimitScript string

//go:embed lua/zclaim_by_score.lua
var zclaimByScoreScript string

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

func (c *redisOrderCache) MGet(ctx context.Context, keys ...string) ([]*string, error) {
	if len(keys) == 0 {
		return []*string{}, nil
	}
	result, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	strings := make([]*string, len(result))
	for i, v := range result {
		if v == nil {
			continue
		}
		str := v.(string)
		strings[i] = &str
	}
	return strings, nil
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

func (c *redisOrderCache) ZAddWithLimit(ctx context.Context, key string, members map[string]float64, limit int64, ttl time.Duration) error {
	if len(members) == 0 {
		return nil
	}

	args := make([]interface{}, 0, 2+len(members)*2)
	args = append(args, limit, int64(ttl.Seconds()))
	for member, score := range members {
		args = append(args, score, member)
	}

	return c.client.Eval(ctx, zaddWithLimitScript, []string{key}, args...).Err()
}

func (c *redisOrderCache) ZRange(ctx context.Context, key string, start, stop int64, reverse bool) ([]string, error) {
	if reverse {
		return c.client.ZRevRange(ctx, key, start, stop).Result()
	}
	return c.client.ZRange(ctx, key, start, stop).Result()
}

func (c *redisOrderCache) ZRangeByScore(ctx context.Context, key, min, max string, limit int64) ([]string, error) {
	args := &redis.ZRangeBy{
		Min: min,
		Max: max,
	}
	if limit > 0 {
		args.Offset = 0
		args.Count = limit
	}
	return c.client.ZRangeByScore(ctx, key, args).Result()
}

func (c *redisOrderCache) ZClaimByScore(ctx context.Context, key, max string, limit int64) ([]string, error) {
	res, err := c.client.Eval(ctx, zclaimByScoreScript, []string{key}, max, limit).Result()
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}

	rawMembers, ok := res.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected redis eval result type %T", res)
	}

	members := make([]string, 0, len(rawMembers))
	for _, raw := range rawMembers {
		switch v := raw.(type) {
		case string:
			members = append(members, v)
		case []byte:
			members = append(members, string(v))
		default:
			return nil, fmt.Errorf("unexpected redis member type %T", raw)
		}
	}
	return members, nil
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
