package main

import (
	"fmt"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
)

func main() {
	initViperWatch()

	svr := InitApp()
	
	port := viper.GetInt("grpc.server.port")
	log.Printf("User service starting on port %d...", port)
	if err := svr.Run(); err != nil {
		log.Fatalf("server run error: %v", err)
	}
}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/product/config/dev.yaml", "配置文件路径")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
	}

	// 支持环境变量覆盖配置文件（环境变量优先）
	viper.AutomaticEnv()
	// 设置环境变量前缀（可选）
	// viper.SetEnvPrefix("USER_SERVICE")

	// 手动绑定环境变量到配置键
	viper.BindEnv("db.dsn", "DB_DSN")
	viper.BindEnv("redis.addr", "REDIS_ADDR")
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
}
