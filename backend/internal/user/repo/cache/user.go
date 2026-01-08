package cache

import (
	"time"

	"github.com/redis/go-redis/v9"
)

type UserCache interface {
}

type RedisUserCache struct {
	cmd redis.Cmdable
	// 过期时间
	expiration time.Duration
}
