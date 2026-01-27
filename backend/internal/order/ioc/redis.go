package ioc

import (
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func InitRedis() redis.Cmdable {
	addr := viper.GetString("redis.addr")
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}
