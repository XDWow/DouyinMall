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

	log.Printf("order service starting, gRPC port=%d http port=%d",
		viper.GetInt("grpc.server.port"),
		viper.GetInt("http.server.port"))

	app.Cron.Start()
	log.Printf("order cron started")

	for _, consumer := range app.Consumers {
		if err := consumer.Start(); err != nil {
			log.Fatalf("consumer start failed: %v", err)
		}
	}
	log.Printf("order consumers started")

	var mcpCfg ordermcp.Config
	mcpOK := viper.UnmarshalKey("mcp", &mcpCfg) == nil && strings.TrimSpace(mcpCfg.Server.Addr) != ""

	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- app.Server.Run()
	}()

	httpErr := make(chan error, 1)
	go func() {
		httpErr <- app.HTTPServer.Start()
	}()

	if mcpOK {
		mcpHandler, err := app.OrderMCPHandler(mcpCfg)
		if err != nil {
			log.Fatalf("init MCP failed: %v", err)
		}
		mux := http.NewServeMux()
		mux.Handle("/mcp", mcpHandler)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		go func() {
			log.Printf("order MCP listening on %s", mcpCfg.Server.Addr)
			if err := http.ListenAndServe(mcpCfg.Server.Addr, mux); err != nil {
				log.Fatalf("MCP HTTP server exited: %v", err)
			}
		}()
	}

	select {
	case err := <-grpcErr:
		if err != nil {
			log.Fatalf("gRPC server exited: %v", err)
		}
	case err := <-httpErr:
		if err != nil {
			log.Fatalf("HTTP server exited: %v", err)
		}
	}
}

func initViperWatch() {
	cfile := pflag.String("config", "internal/order/config/dev.yaml", "config file path")
	pflag.Parse()

	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %w", err))
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("ORDER")
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("rocketmq.endpoint", "ROCKETMQ_ENDPOINT")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
}
