package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
		fmt.Printf("BFF HTTP 鏈嶅姟鍚姩鍦? %s\n", addr)
		if err := app.Server.Start(); err != nil {
			panic(fmt.Errorf("HTTP 鏈嶅姟鍚姩澶辫触: %w", err))
		}
	}()

	// 浼橀泤閫€鍑?
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("BFF 鏈嶅姟宸插叧闂?)
}

func initViper() {
	cfile := pflag.String("config",
		"internal/bff/config/dev.yaml", "閰嶇疆鏂囦欢璺緞")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("璇诲彇閰嶇疆鏂囦欢澶辫触: %w", err))
	}

	viper.AutomaticEnv()
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	viper.BindEnv("http.addr", "HTTP_ADDR")
	viper.BindEnv("jwt.access_secret", "JWT_ACCESS_SECRET")
}


