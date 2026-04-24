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
	log.Printf("Search service starting on port %d...", port)

	for i, consumer := range app.Consumers {
		if err := consumer.Start(); err != nil {
			log.Fatalf("Consumer %d 鍚姩澶辫触: %v", i+1, err)
		}
		log.Printf("Consumer %d started", i+1)
	}

	// 鍚姩 gRPC Server锛堥樆濉烇級
	if err := app.Server.Run(); err != nil {
		log.Fatalf("server run error: %v", err)
	}
}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/search/config/dev.yaml", "閰嶇疆鏂囦欢璺緞")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("璇诲彇閰嶇疆鏂囦欢澶辫触: %w", err))
	}

	// 鏀寔鐜鍙橀噺瑕嗙洊閰嶇疆鏂囦欢锛堢幆澧冨彉閲忎紭鍏堬級
	viper.AutomaticEnv()

	// 鎵嬪姩缁戝畾鐜鍙橀噺鍒伴厤缃敭
	viper.BindEnv("db.password", "DB_PASSWORD")
	viper.BindEnv("elasticsearch.addresses", "ES_ADDRESSES")
	viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	viper.BindEnv("grpc.server.port", "GRPC_PORT")
	viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
}
