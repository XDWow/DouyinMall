package cache

import (
	"time"

	"github.com/redis/go-redis/v9"
)

type UserCache interface {
	// 鏆傛椂涓虹┖锛屽悗缁寜闇€娣诲姞缂撳瓨鏂规硶
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


