package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	ordermcp "github.com/XDWow/DouyinMall/backend/internal/order/transport/mcp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	app := InitApp()

	grpcPort := viper.GetInt("grpc.server.port")
	log.Printf("订单服务启动，gRPC 端口 %d", grpcPort)

	app.Cron.Start()
	log.Printf("定时任务已启动")

	for _, consumer := range app.Consumers {
		if err := consumer.Start(); err != nil {
			log.Fatalf("consumer start failed: %v", err)
		}
	}
	log.Printf("异步消费者已启动")

	var mcpCfg ordermcp.Config
	mcpOK := viper.UnmarshalKey("mcp", &mcpCfg) == nil && strings.TrimSpace(mcpCfg.Server.Addr) != ""

	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- app.Server.Run()
	}()

	if mcpOK {
		mcpHandler, err := app.OrderMCPHandler(mcpCfg)
		if err != nil {
			log.Fatalf("初始化 MCP 失败: %v", err)
		}
		mux := http.NewServeMux()
		mux.Handle("/mcp", mcpHandler)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		go func() {
			log.Printf("订单 MCP 已启动，监听 %s（路径 /mcp）", mcpCfg.Server.Addr)
			if err := http.ListenAndServe(mcpCfg.Server.Addr, mux); err != nil {
				log.Fatalf("MCP HTTP 服务异常退出: %v", err)
			}
		}()
	}

	if err := <-grpcErr; err != nil {
		log.Fatalf("gRPC 服务退出: %v", err)
	}
}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/order/config/dev.yaml", "配置文件路径")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
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
