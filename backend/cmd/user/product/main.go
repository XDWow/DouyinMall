package product

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()
}

func initViperWatch() {
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	cfile := pflag.String("config", "internal/user/config/dev.yaml", "config file path")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %w", err))
	}

	viper.AutomaticEnv()
	viper.BindEnv("db.password", "DB_PASSWORD")
	viper.BindEnv("redis.addr", "REDIS_ADDR")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
}
