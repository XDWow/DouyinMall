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

	go func() {
		addr := viper.GetString("http.addr")
		if addr == "" {
			addr = ":8080"
		}
		fmt.Printf("BFF HTTP server listening on %s\n", addr)
		if err := app.Server.Start(); err != nil {
			panic(fmt.Errorf("HTTP server exited with error: %w", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("BFF server stopped")
}

func initViper() {
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	configFile := pflag.String("config", "internal/bff/config/dev.yaml", "BFF config file path")
	pflag.Parse()

	viper.SetConfigFile(*configFile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %w", err))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("http.addr", "HTTP_ADDR")
	_ = viper.BindEnv("jwt.access_secret", "JWT_ACCESS_SECRET")
	_ = viper.BindEnv("jwt.refresh_secret", "JWT_REFRESH_SECRET")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
}
