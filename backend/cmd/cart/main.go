package main

import (
	"fmt"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	// app := InitApp()

	// port := viper.GetInt("grpc.server.port")
	// log.Printf("Search service starting on port %d...", port)

	// // 启动 gRPC Server（阻塞）
	// if err := app.Server.Run(); err != nil {
	// 	log.Fatalf("server run error: %v", err)
	// }
}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/search/config/dev.yaml", "配置文件路径")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
	}

	// 支持环境变量覆盖配置文件（环境变量优先）
	viper.AutomaticEnv()

	// 手动绑定环境变量到配置键
	viper.BindEnv("elasticsearch.addresses", "ES_ADDRESSES")
	viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	viper.BindEnv("grpc.server.port", "GRPC_PORT")
	viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
}