package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()
	app := InitApp()

	go func() {
		port := viper.GetInt("grpc.server.port")
		log.Printf("支付 gRPC 服务启动中，端口 %d...", port)
		if err := app.GRPCServer.Run(); err != nil {
			log.Fatalf("gRPC 服务运行失败: %v", err)
		}
	}()

	go func() {
		port := viper.GetInt("http.server.port")
		log.Printf("支付 HTTP 服务启动中，端口 %d...", port)
		if err := app.HTTPServer.Start(); err != nil {
			log.Fatalf("HTTP 服务运行失败: %v", err)
		}
	}()

	app.Cron.Start()
	log.Println("支付定时任务已启动")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭支付服务...")

	stopCtx := app.Cron.Stop()
	<-stopCtx.Done()

	if err := app.GRPCServer.Stop(); err != nil {
		log.Printf("gRPC 服务强制关闭: %v", err)
	}

	log.Println("支付服务已退出")
}

func initViperWatch() {
	cwd, _ := os.Getwd()
	configPath := pflag.String("config", "internal/payment/config/dev.yaml", "payment config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read payment config failed: %w (cwd: %s)", err, cwd))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("rocketmq.endpoint", "ROCKETMQ_ENDPOINT")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")
	_ = viper.BindEnv("payment.provider", "PAYMENT_PROVIDER")
	_ = viper.BindEnv("alipay.app_id", "ALIPAY_APP_ID")
	_ = viper.BindEnv("alipay.private_key", "ALIPAY_PRIVATE_KEY")
	_ = viper.BindEnv("alipay.public_key", "ALIPAY_PUBLIC_KEY")
	_ = viper.BindEnv("alipay.notify_url", "ALIPAY_NOTIFY_URL")
	viper.WatchConfig()

	log.Println("支付配置已加载:", viper.ConfigFileUsed())
}
