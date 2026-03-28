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
		log.Printf("Payment gRPC service starting on port %d...", port)
		if err := app.GRPCServer.Run(); err != nil {
			log.Fatalf("gRPC server run error: %v", err)
		}
	}()

	go func() {
		port := viper.GetInt("http.server.port")
		log.Printf("Payment HTTP service (for wechat callback) starting on port %d...", port)
		if err := app.HTTPServer.Start(); err != nil {
			log.Fatalf("HTTP server run error: %v", err)
		}
	}()

	app.Cron.Start()
	log.Println("Payment cron jobs started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down payment services...")

	stopCtx := app.Cron.Stop()
	<-stopCtx.Done()

	if err := app.GRPCServer.Stop(); err != nil {
		log.Printf("gRPC server forced to shutdown: %v", err)
	}

	log.Println("Payment services exited")
}

func initViperWatch() {
	cwd, _ := os.Getwd()
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/payment/config")
	viper.AddConfigPath("../internal/payment/config")
	viper.AddConfigPath("../../internal/payment/config")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w (工作目录: %s)", err, cwd))
	}

	viper.WatchConfig()

	log.Println("配置文件加载成功:", viper.ConfigFileUsed())
}
