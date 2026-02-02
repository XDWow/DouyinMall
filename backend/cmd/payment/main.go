package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
)

func main() {
	initViperWatch()
	app := InitApp()

	// 优雅启动和关闭
	go func() {
		port := viper.GetInt("grpc.server.port")
		log.Printf("Payment gRPC service starting on port %d...", port)
		if err := app.GRPCServer.Run(); err != nil {
			log.Fatalf("gRPC server run error: %v", err)
		}
	}()

	go func() {
		port := viper.GetInt("http.server.port")
		log.Printf("Payment HTTP service (for wechat callback) starting on port %d...", port)
		if err := app.HTTPServer.Start(); err != nil {
			log.Fatalf("HTTP server run error: %v", err)
		}
	}()

	// 启动定时任务（每30分钟执行一次）
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		log.Printf("Payment sync job [%s] starting...", app.Job.Name())

		// 启动时立即执行一次
		if err := app.Job.Run(); err != nil {
			log.Printf("Job [%s] execution error: %v", app.Job.Name(), err)
		}

		for range ticker.C {
			if err := app.Job.Run(); err != nil {
				log.Printf("Job [%s] execution error: %v", app.Job.Name(), err)
			}
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	// 优雅关闭 gRPC 服务器
	if err := app.GRPCServer.Stop(); err != nil {
		log.Printf("gRPC server forced to shutdown: %v", err)
	}

	log.Println("Servers exited")
}

// initViperWatch 初始化 Viper 配置
func initViperWatch() {
	cwd, _ := os.Getwd()
	viper.SetConfigName("dev")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./internal/payment/config")
	viper.AddConfigPath("../internal/payment/config")
	viper.AddConfigPath("../../internal/payment/config")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w (工作目录: %s)", err, cwd))
	}

	// 监听配置文件变化
	viper.WatchConfig()

	log.Println("配置文件加载成功:", viper.ConfigFileUsed())
}
