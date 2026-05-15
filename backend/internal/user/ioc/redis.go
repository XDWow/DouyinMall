package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/user/config"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func InitRedis() redis.Cmdable {
	c := config.RedisConfig{
		Addr: "localhost:6379",
	}
	_ = viper.UnmarshalKey("redis", &c)
	return redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
	})
}
