package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
)

func main() {
	initViperWatch()
	app := InitApp()

	go func() {
		port := viper.GetInt("grpc.server.port")
		log.Printf("Checkout gRPC service starting on port %d...", port)
		if err := app.GRPCServer.Run(); err != nil {
			log.Fatalf("gRPC server run error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down checkout service...")
	if err := app.GRPCServer.Stop(); err != nil {
		log.Printf("gRPC server forced to shutdown: %v", err)
	}
	log.Println("Checkout service exited")
}

func initViperWatch() {
	cwd, _ := os.Getwd()
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/checkout/config")
	viper.AddConfigPath("../internal/checkout/config")
	viper.AddConfigPath("../../internal/checkout/config")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("璇诲彇閰嶇疆鏂囦欢澶辫触: %w (宸ヤ綔鐩綍: %s)", err, cwd))
	}
}


