package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViper()
	app := InitApp()
	if err := app.SeckillConsumer.Start(); err != nil {
		panic(err)
	}
	if err := app.OrderStatusConsumer.Start(); err != nil {
		panic(err)
	}
	go func() {
		if err := app.GRPCServer.Run(); err != nil {
			panic(fmt.Errorf("seckill grpc run failed: %w", err))
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	_ = app.GRPCServer.Stop()
}

func initViper() {
	configPath := pflag.String("config", "internal/seckill/config/dev.yaml", "seckill config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
}
