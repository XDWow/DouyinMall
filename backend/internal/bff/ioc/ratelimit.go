package ioc

import (
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// InitRateLimiter 鍒濆鍖栧熀浜?Redis 婊戝姩绐楀彛鐨?IP 闄愭祦鍣?
// 閰嶇疆椤癸紙dev.yaml锛夛細
//
//	ratelimit.interval: 绐楀彛澶у皬锛岄粯璁?1m
//	ratelimit.rate:     绐楀彛鍐呮渶澶氳姹傛暟锛岄粯璁?100
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


