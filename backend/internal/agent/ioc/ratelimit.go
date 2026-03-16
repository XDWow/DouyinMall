package ioc

import (
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// InitSystemRateLimiter 系统总限流器（Redis 滑动窗口）
//
// 依赖 viper key：
//
//	agent.system_rpm  整个 agent 服务每分钟最多处理请求数（默认 500）
//
// 策略：Redis 故障时降级放行（调用方用 err != nil 判断），不阻塞正常业务。
func InitSystemRateLimiter(cmd redis.Cmdable) ratelimit.Limiter {
	rpm := viper.GetInt("agent.system_rpm")
	if rpm == 0 {
		rpm = 500
	}
	return ratelimit.NewRedisSlidingWindowLimiter(cmd, time.Minute, rpm)
}

// InitUserRateLimiter 用户维度限流器（Redis 滑动窗口）
//
// 依赖 viper key：
//
//	agent.user_rpm  单用户每分钟最多发送消息数（默认 10）
//
// key 格式：agent:rate:<userID>，由 ChatUseCase 在调用时注入。
func InitUserRateLimiter(cmd redis.Cmdable) ratelimit.Limiter {
	rpm := viper.GetInt("agent.user_rpm")
	if rpm == 0 {
		rpm = 10
	}
	return ratelimit.NewRedisSlidingWindowLimiter(cmd, time.Minute, rpm)
}
