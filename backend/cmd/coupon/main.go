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

	// 鍚姩Kafka娑堣垂鑰咃紙璁㈠崟鐘舵€佸彉鏇达級
	if err := app.OrderConsumer.Start(); err != nil {
		fmt.Printf("璀﹀憡: Kafka娑堣垂鑰呭惎鍔ㄥけ璐? %v锛岀户缁繍琛孿n", err)
	} else {
		fmt.Println("Kafka娑堣垂鑰呭凡鍚姩")
	}

	// 鍚姩gRPC鏈嶅姟
	go func() {
		fmt.Printf("Coupon gRPC鏈嶅姟鍚姩鍦? %d\n", viper.GetInt("grpc.server.port"))
		if err := app.GRPCServer.Run(); err != nil {
			panic(fmt.Errorf("gRPC鏈嶅姟鍚姩澶辫触: %w", err))
		}
	}()

	// 浼橀泤閫€鍑?	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("姝ｅ湪鍏抽棴Coupon鏈嶅姟...")
	if err := app.GRPCServer.Stop(); err != nil {
		fmt.Printf("gRPC鏈嶅姟鍏抽棴澶辫触: %v\n", err)
	}
	fmt.Println("Coupon鏈嶅姟宸插叧闂?)
}

func initViper() {
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/coupon/config")
	viper.AddConfigPath(".")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("璀﹀憡: 璇诲彇閰嶇疆鏂囦欢澶辫触: %v锛屼娇鐢ㄩ粯璁ら厤缃甛n", err)
	}
}


