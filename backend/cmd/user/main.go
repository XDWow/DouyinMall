package main

import (
	"fmt"
	"log"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	svr := InitApp()

	port := viper.GetInt("grpc.server.port")
	log.Printf("User service starting on port %d...", port)
	if err := svr.Run(); err != nil {
		log.Fatalf("server run error: %v", err)
	}
}

func initViperWatch() {
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	cfile := pflag.String("config", "internal/user/config/dev.yaml", "user config file path")
	pflag.Parse()

	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read user config failed: %w", err))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
}
