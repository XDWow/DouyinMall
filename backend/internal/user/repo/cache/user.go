package cache

import (
	"time"

	"github.com/redis/go-redis/v9"
)

type UserCache interface {
	// 暂时为空，后续按需添加缓存方法。
}

type RedisUserCache struct {
	cmd        redis.Cmdable
	expiration time.Duration
}

func NewRedisUserCache(cmd redis.Cmdable) UserCache {
	return &RedisUserCache{
		cmd:        cmd,
		expiration: 15 * time.Minute,
	}
}
