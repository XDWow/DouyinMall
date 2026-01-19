package cache

import (
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisCartCache struct {
	cmd        redis.Cmdable
	expiration time.Duration
}
