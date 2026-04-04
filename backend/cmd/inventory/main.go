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
		fmt.Printf("warning: failed to start order consumer: %v\n", err)
	} else {
		fmt.Println("order consumer started")
	}

	go func() {
		fmt.Printf("inventory grpc server listening on %d\n", viper.GetInt("grpc.server.port"))
		if err := app.GRPCServer.Run(); err != nil {
			panic(fmt.Errorf("start inventory grpc server: %w", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("shutting down inventory service")
	if err := app.GRPCServer.Stop(); err != nil {
		fmt.Printf("stop inventory grpc server failed: %v\n", err)
	}
}

func initViper() {
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/inventory/config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("warning: read inventory config failed: %v\n", err)
	}
}


