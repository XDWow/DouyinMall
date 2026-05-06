package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/coupon/config")
	viper.AddConfigPath(".")

	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("rocketmq.endpoint", "ROCKETMQ_ENDPOINT")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("warning: read coupon config failed: %v\n", err)
	}
}
