package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	inventorymcp "github.com/XDWow/DouyinMall/backend/internal/inventory/transport/mcp"
	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViper()

	app := InitApp()

	if err := app.OrderConsumer.Start(); err != nil {
		fmt.Printf("warning: failed to start order consumer: %v\n", err)
	} else {
		fmt.Println("order consumer started")
	}

	grpcErr := make(chan error, 1)
	go func() {
		fmt.Printf("inventory grpc server listening on %d\n", viper.GetInt("grpc.server.port"))
		grpcErr <- app.GRPCServer.Run()
	}()

	var mcpErr <-chan error
	var mcpCfg inventorymcp.Config
	if viper.UnmarshalKey("mcp", &mcpCfg) == nil && strings.TrimSpace(mcpCfg.Server.Addr) != "" {
		mcpHandler, err := newMCPHandler(mcpCfg)
		if err != nil {
			panic(fmt.Errorf("init inventory MCP failed: %w", err))
		}
		mux := http.NewServeMux()
		mux.Handle("/mcp", mcpHandler)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		mcpErrCh := make(chan error, 1)
		mcpErr = mcpErrCh
		go func() {
			fmt.Printf("inventory MCP listening on %s\n", mcpCfg.Server.Addr)
			mcpErrCh <- http.ListenAndServe(mcpCfg.Server.Addr, mux)
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		fmt.Println("shutting down inventory service")
		if err := app.GRPCServer.Stop(); err != nil {
			fmt.Printf("stop inventory grpc server failed: %v\n", err)
		}
	case err := <-grpcErr:
		if err != nil {
			panic(fmt.Errorf("start inventory grpc server: %w", err))
		}
	case err := <-mcpErr:
		if err != nil {
			panic(fmt.Errorf("start inventory MCP server: %w", err))
		}
	}
}

func initViper() {
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	configPath := pflag.String("config", "internal/inventory/config/dev.yaml", "inventory config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("rocketmq.endpoint", "ROCKETMQ_ENDPOINT")
	_ = viper.BindEnv("rocketmq.access_key", "ROCKETMQ_ACCESS_KEY")
	_ = viper.BindEnv("rocketmq.secret_key", "ROCKETMQ_SECRET_KEY")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("mcp.server.addr", "MCP_ADDR")
	_ = viper.BindEnv("mcp.upstream.direct_addr", "MCP_UPSTREAM_DIRECT_ADDR")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read inventory config failed: %w", err))
	}

	if endpoints := strings.TrimSpace(os.Getenv("ETCD_ENDPOINTS")); endpoints != "" {
		parts := strings.Split(endpoints, ",")
		cleaned := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				cleaned = append(cleaned, part)
			}
		}
		if len(cleaned) > 0 {
			viper.Set("etcd.endpoints", cleaned)
		}
	}
}
