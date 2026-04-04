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
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/seckill/config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
}


