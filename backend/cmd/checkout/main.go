package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
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

	go func() {
		port := viper.GetInt("http.server.port")
		log.Printf("Checkout HTTP service starting on port %d...", port)
		if err := app.HTTPServer.Start(); err != nil {
			log.Fatalf("HTTP server run error: %v", err)
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
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	configPath := pflag.String("config", "internal/checkout/config/dev.yaml", "checkout config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read checkout config failed: %w (cwd: %s)", err, cwd))
	}
}
