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

	// 鍚姩 Kafka 娑堣垂鑰咃紙鐩戝惉璁㈠崟鏀粯鎴愬姛浜嬩欢锛屾竻鐞嗚喘鐗╄溅锛?
	if err := app.OrderConsumer.Start(); err != nil {
		fmt.Printf("璀﹀憡: Kafka娑堣垂鑰呭惎鍔ㄥけ璐? %v锛岀户缁繍琛孿n", err)
	} else {
		fmt.Println("Cart OrderConsumer宸插惎鍔?)
	}

	go func() {
		fmt.Printf("Cart gRPC鏈嶅姟鍚姩鍦? %d\n", viper.GetInt("grpc.server.port"))
		if err := app.Server.Run(); err != nil {
			panic(fmt.Errorf("gRPC鏈嶅姟鍚姩澶辫触: %w", err))
		}
	}()

	// 浼橀泤閫€鍑?
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("姝ｅ湪鍏抽棴Cart鏈嶅姟...")
	if err := app.OrderConsumer.Stop(); err != nil {
		fmt.Printf("鍏抽棴 Kafka 娑堣垂鑰呭け璐? %v\n", err)
	}
	if err := app.Server.Stop(); err != nil {
		fmt.Printf("鍏抽棴 gRPC 鏈嶅姟澶辫触: %v\n", err)
	}
	fmt.Println("Cart鏈嶅姟宸插叧闂?)
}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/cart/config/dev.yaml", "閰嶇疆鏂囦欢璺緞")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("璇诲彇閰嶇疆鏂囦欢澶辫触: %w", err))
	}

	// 鏀寔鐜鍙橀噺瑕嗙洊閰嶇疆鏂囦欢锛堢幆澧冨彉閲忎紭鍏堬級
	viper.AutomaticEnv()

	viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	viper.BindEnv("grpc.server.port", "GRPC_PORT")
	viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
}


