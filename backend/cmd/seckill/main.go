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
	if err := app.DeadLetterConsumer.Start(); err != nil {
		panic(err)
	}
	if err := app.OrderStatusConsumer.Start(); err != nil {
		panic(err)
	}

	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- app.GRPCServer.Run()
	}()

	httpErr := make(chan error, 1)
	go func() {
		httpErr <- app.HTTPServer.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		_ = app.SeckillConsumer.Stop()
		_ = app.DeadLetterConsumer.Stop()
		_ = app.OrderStatusConsumer.Stop()
		_ = app.Producer.Stop()
		_ = app.GRPCServer.Stop()
	case err := <-grpcErr:
		if err != nil {
			panic(fmt.Errorf("seckill grpc run failed: %w", err))
		}
	case err := <-httpErr:
		if err != nil {
			panic(fmt.Errorf("seckill http run failed: %w", err))
		}
	}
}

func initViper() {
	configPath := pflag.String("config", "internal/seckill/config/dev.yaml", "seckill config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("rocketmq.endpoint", "ROCKETMQ_ENDPOINT")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")
	_ = viper.BindEnv("snowflake.node_id", "SNOWFLAKE_NODE_ID")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
}
