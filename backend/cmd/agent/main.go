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
		fmt.Printf("Agent gRPC 服务启动在: %d\n", viper.GetInt("grpc.server.port"))
		if err := app.Server.Run(); err != nil {
			panic(fmt.Errorf("gRPC 服务启动失败: %w", err))
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("正在关闭 Agent 服务...")
	if err := app.Server.Stop(); err != nil {
		fmt.Printf("关闭 gRPC 服务失败: %v\n", err)
	}
	fmt.Println("Agent 服务已关闭")
}

func initViper() {
	cfile := pflag.String("config",
		"internal/agent/config/dev.yaml", "配置文件路径")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
	}

	viper.AutomaticEnv()
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	viper.BindEnv("grpc.server.port", "GRPC_PORT")
	viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
	viper.BindEnv("llm.api_key", "LLM_API_KEY")
	viper.BindEnv("embedding.api_key", "EMBEDDING_API_KEY")
}
