package ioc

import (
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// InitRedis 鍒濆鍖?Redis 瀹㈡埛绔紙鐢ㄤ簬闄愭祦绛夊姛鑳斤級
func InitRedis() redis.Cmdable {
	addr := viper.GetString("redis.addr")
	if addr == "" {
		addr = "localhost:6379"
	}
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}


