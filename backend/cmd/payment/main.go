package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()
	app := InitApp()

	go func() {
		port := viper.GetInt("grpc.server.port")
		log.Printf("payment gRPC service starting on port %d...", port)
		if err := app.GRPCServer.Run(); err != nil {
			log.Fatalf("payment gRPC server failed: %v", err)
		}
	}()

	go func() {
		port := viper.GetInt("http.server.port")
		log.Printf("payment HTTP service starting on port %d...", port)
		if err := app.HTTPServer.Start(); err != nil {
			log.Fatalf("payment HTTP server failed: %v", err)
		}
	}()

	app.Cron.Start()
	log.Println("payment cron started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down payment service...")

	stopCtx := app.Cron.Stop()
	<-stopCtx.Done()

	if err := app.GRPCServer.Stop(); err != nil {
		log.Printf("payment gRPC server forced to stop: %v", err)
	}

	log.Println("payment service exited")
}

func initViperWatch() {
	cwd, _ := os.Getwd()
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	configPath := pflag.String("config", "internal/payment/config/dev.yaml", "payment config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read payment config failed: %w (cwd: %s)", err, cwd))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
	_ = viper.BindEnv("http.server.port", "HTTP_PORT")
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("rocketmq.endpoint", "ROCKETMQ_ENDPOINT")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")
	_ = viper.BindEnv("payment.provider", "PAYMENT_PROVIDER")
	_ = viper.BindEnv("wechat.api_base_url", "WECHAT_API_BASE_URL")
	_ = viper.BindEnv("wechat.app_id", "WECHAT_APP_ID")
	_ = viper.BindEnv("wechat.mch_id", "WECHAT_MCH_ID")
	_ = viper.BindEnv("wechat.cert_serial_no", "WECHAT_CERT_SERIAL_NO")
	_ = viper.BindEnv("wechat.private_key_path", "WECHAT_PRIVATE_KEY_PATH")
	_ = viper.BindEnv("wechat.api_v3_key", "WECHAT_API_V3_KEY")
	_ = viper.BindEnv("wechat.notify_url", "WECHAT_NOTIFY_URL")
	_ = viper.BindEnv("alipay.app_id", "ALIPAY_APP_ID")
	_ = viper.BindEnv("alipay.pid", "ALIPAY_PID")
	_ = viper.BindEnv("alipay.gateway", "ALIPAY_GATEWAY")
	_ = viper.BindEnv("alipay.private_key", "ALIPAY_PRIVATE_KEY")
	_ = viper.BindEnv("alipay.public_key", "ALIPAY_PUBLIC_KEY")
	_ = viper.BindEnv("alipay.sandbox", "ALIPAY_SANDBOX")
	_ = viper.BindEnv("alipay.notify_url", "ALIPAY_NOTIFY_URL")
	_ = viper.BindEnv("alipay.return_url", "ALIPAY_RETURN_URL")
	viper.WatchConfig()

	log.Printf("payment config loaded from %s", viper.ConfigFileUsed())
}
