package ratelimit

import (
	"context"
	_ "embed"
	"github.com/redis/go-redis/v9"
	"time"
)

//go:embed slide_window.lua
var luaSlideWindow string

// RedisSlidingWindowLimiter Redis 涓婄殑婊戝姩绐楀彛绠楁硶闄愭祦鍣ㄥ疄鐜?
type RedisSlidingWindowLimiter struct {
	cmd redis.Cmdable

	// 绐楀彛澶у皬
	interval time.Duration
	// 闃堝€?
	rate int
	// interval 鍐呭厑璁?rate 涓姹?
	// 1s 鍐呭厑璁?3000 涓姹?
}

func NewRedisSlidingWindowLimiter(cmd redis.Cmdable,
	interval time.Duration, rate int) Limiter {
	return &RedisSlidingWindowLimiter{
		cmd:      cmd,
		interval: interval,
		rate:     rate,
	}
}

func (r *RedisSlidingWindowLimiter) Limit(ctx context.Context, key string) (bool, error) {
	return r.cmd.Eval(ctx, luaSlideWindow, []string{key},
		r.interval.Milliseconds(), r.rate, time.Now().UnixMilli()).Bool()
}


