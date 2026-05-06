package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	app := InitApp()

	if err := app.OrderConsumer.Start(); err != nil {
		log.Printf("cart order consumer start failed: %v", err)
	} else {
		log.Printf("cart order consumer started")
	}

	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- app.Server.Run()
	}()

	httpErr := make(chan error, 1)
	go func() {
		httpErr <- app.HTTPServer.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		if err := app.OrderConsumer.Stop(); err != nil {
			log.Printf("stop cart order consumer failed: %v", err)
		}
		if err := app.Server.Stop(); err != nil {
			log.Printf("stop cart grpc failed: %v", err)
		}
	case err := <-grpcErr:
		if err != nil {
			panic(fmt.Errorf("cart grpc run failed: %w", err))
		}
	case err := <-httpErr:
		if err != nil {
			panic(fmt.Errorf("cart http run failed: %w", err))
		}
	}
}

func initViperWatch() {
	cfile := pflag.String("config", "internal/cart/config/dev.yaml", "config file path")
	pflag.Parse()

	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %w", err))
	}

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
}
