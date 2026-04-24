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

	go func() {
		port := viper.GetInt("grpc.server.port")
		log.Printf("Payment gRPC service starting on port %d...", port)
		if err := app.GRPCServer.Run(); err != nil {
			log.Fatalf("gRPC server run error: %v", err)
		}
	}()

	go func() {
		port := viper.GetInt("http.server.port")
		log.Printf("Payment HTTP service starting on port %d...", port)
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
	configPath := pflag.String("config", "internal/payment/config/dev.yaml", "payment config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read payment config failed: %w (cwd: %s)", err, cwd))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("payment.provider", "PAYMENT_PROVIDER")
	_ = viper.BindEnv("alipay.app_id", "ALIPAY_APP_ID")
	_ = viper.BindEnv("alipay.private_key", "ALIPAY_PRIVATE_KEY")
	_ = viper.BindEnv("alipay.public_key", "ALIPAY_PUBLIC_KEY")
	_ = viper.BindEnv("alipay.notify_url", "ALIPAY_NOTIFY_URL")
	viper.WatchConfig()

	log.Println("payment config loaded:", viper.ConfigFileUsed())
}
