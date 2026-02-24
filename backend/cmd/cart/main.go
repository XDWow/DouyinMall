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
	initViperWatch()

	app := InitApp()

	// 启动 Kafka 消费者（监听订单支付成功事件，清理购物车）
	if err := app.OrderConsumer.Start(); err != nil {
		fmt.Printf("警告: Kafka消费者启动失败: %v，继续运行\n", err)
	} else {
		fmt.Println("Cart OrderConsumer已启动")
	}

	go func() {
		fmt.Printf("Cart gRPC服务启动在: %d\n", viper.GetInt("grpc.server.port"))
		if err := app.Server.Run(); err != nil {
			panic(fmt.Errorf("gRPC服务启动失败: %w", err))
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("正在关闭Cart服务...")
	if err := app.OrderConsumer.Stop(); err != nil {
		fmt.Printf("关闭 Kafka 消费者失败: %v\n", err)
	}
	if err := app.Server.Stop(); err != nil {
		fmt.Printf("关闭 gRPC 服务失败: %v\n", err)
	}
	fmt.Println("Cart服务已关闭")
}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/cart/config/dev.yaml", "配置文件路径")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
	}

	// 支持环境变量覆盖配置文件（环境变量优先）
	viper.AutomaticEnv()

	viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	viper.BindEnv("grpc.server.port", "GRPC_PORT")
	viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
}
