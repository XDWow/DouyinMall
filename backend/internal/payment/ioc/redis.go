package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/payment/config"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func InitRedis() redis.Cmdable {
	c := config.RedisConfig{
		Addr: "localhost:6379",
	}
	_ = viper.UnmarshalKey("redis", &c)
	c.Password = viper.GetString("redis.password")

	return redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})
}
