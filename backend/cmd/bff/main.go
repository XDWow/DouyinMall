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
		fmt.Printf("BFF HTTP 服务启动在: %s\n", addr)
		if err := app.Server.Start(); err != nil {
			panic(fmt.Errorf("HTTP 服务启动失败: %w", err))
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("BFF 服务已关闭")
}

func initViper() {
	cfile := pflag.String("config",
		"internal/bff/config/dev.yaml", "配置文件路径")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
	}

	viper.AutomaticEnv()
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	viper.BindEnv("http.addr", "HTTP_ADDR")
	viper.BindEnv("jwt.access_secret", "JWT_ACCESS_SECRET")
}
