package main

import (
	"fmt"
	"log"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	app := InitApp()

	port := viper.GetInt("grpc.server.port")
	log.Printf("Order service starting on port %d...", port)

	app.Cron.Start()
	log.Printf("瀹氭椂浠诲姟宸插惎鍔?)

	for _, consumer := range app.Consumers {
		if err := consumer.Start(); err != nil {
			log.Fatalf("consumer start failed: %v", err)
		}
	}
	log.Printf("寮傛娑堣垂鑰呭凡鍚姩")

	if err := app.Server.Run(); err != nil {
		log.Fatalf("server run error: %v", err)
	}
}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/order/config/dev.yaml", "閰嶇疆鏂囦欢璺緞")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("璇诲彇閰嶇疆鏂囦欢澶辫触: %w", err))
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("ORDER")
	_ = viper.BindEnv("db.dsn", "DB_DSN")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
}


