package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViper()

	app := InitApp()

	if err := app.OrderConsumer.Start(); err != nil {
		fmt.Printf("warning: coupon order consumer start failed: %v\n", err)
	} else {
		fmt.Println("coupon order consumer started")
	}

	go func() {
		fmt.Printf("coupon grpc server listening on %d\n", viper.GetInt("grpc.server.port"))
		if err := app.GRPCServer.Run(); err != nil {
			panic(fmt.Errorf("start coupon grpc server: %w", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("shutting down coupon service")
	if err := app.GRPCServer.Stop(); err != nil {
		fmt.Printf("stop coupon grpc server failed: %v\n", err)
	}
}

func initViper() {
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	configPath := pflag.String("config", "internal/coupon/config/dev.yaml", "coupon config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("rocketmq.endpoint", "ROCKETMQ_ENDPOINT")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read coupon config failed: %w", err))
	}

	if endpoints := strings.TrimSpace(os.Getenv("ETCD_ENDPOINTS")); endpoints != "" {
		parts := strings.Split(endpoints, ",")
		cleaned := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				cleaned = append(cleaned, part)
			}
		}
		if len(cleaned) > 0 {
			viper.Set("etcd.endpoints", cleaned)
		}
	}
}
