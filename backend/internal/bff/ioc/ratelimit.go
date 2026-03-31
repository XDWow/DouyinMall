package ioc

import (
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// InitRateLimiter 初始化基于 Redis 滑动窗口的 IP 限流器
// 配置项（dev.yaml）：
//
//	ratelimit.interval: 窗口大小，默认 1m
//	ratelimit.rate:     窗口内最多请求数，默认 100
func InitRateLimiter(cmd redis.Cmdable) ratelimit.Limiter {
	interval := viper.GetDuration("ratelimit.interval")
	if interval == 0 {
		interval = time.Minute
	}
	rate := viper.GetInt("ratelimit.rate")
	if rate == 0 {
		rate = 100
	}
	return ratelimit.NewRedisSlidingWindowLimiter(cmd, interval, rate)
}
