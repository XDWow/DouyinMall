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

	// 启动Kafka消费者（订单状态变更）
	if err := app.OrderConsumer.Start(); err != nil {
		fmt.Printf("警告: Kafka消费者启动失败: %v，继续运行\n", err)
	} else {
		fmt.Println("Kafka消费者已启动")
	}

	// 启动gRPC服务
	go func() {
		fmt.Printf("Coupon gRPC服务启动在: %d\n", viper.GetInt("grpc.server.port"))
		if err := app.GRPCServer.Run(); err != nil {
			panic(fmt.Errorf("gRPC服务启动失败: %w", err))
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("正在关闭Coupon服务...")
	if err := app.GRPCServer.Stop(); err != nil {
		fmt.Printf("gRPC服务关闭失败: %v\n", err)
	}
	fmt.Println("Coupon服务已关闭")
}

func initViper() {
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/coupon/config")
	viper.AddConfigPath(".")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("警告: 读取配置文件失败: %v，使用默认配置\n", err)
	}
}
