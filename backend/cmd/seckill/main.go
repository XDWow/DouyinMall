package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
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
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	configPath := pflag.String("config", "internal/seckill/config/dev.yaml", "seckill config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
	_ = viper.BindEnv("http.server.port", "HTTP_PORT")
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("rocketmq.name_server", "ROCKETMQ_NAME_SERVER")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")
	_ = viper.BindEnv("rocketmq.producer_group", "ROCKETMQ_PRODUCER_GROUP")
	_ = viper.BindEnv("rocketmq.request_group", "ROCKETMQ_REQUEST_GROUP")
	_ = viper.BindEnv("rocketmq.dead_letter_group", "ROCKETMQ_DEAD_LETTER_GROUP")
	_ = viper.BindEnv("rocketmq.order_status_group", "ROCKETMQ_ORDER_STATUS_GROUP")
	_ = viper.BindEnv("rocketmq.handle_timeout_sec", "ROCKETMQ_HANDLE_TIMEOUT_SEC")
	_ = viper.BindEnv("rocketmq.producer_max_attempts", "ROCKETMQ_PRODUCER_MAX_ATTEMPTS")
	_ = viper.BindEnv("rocketmq.global_worker_num", "ROCKETMQ_GLOBAL_WORKER_NUM")
	_ = viper.BindEnv("snowflake.node_id", "SNOWFLAKE_NODE_ID")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
}
