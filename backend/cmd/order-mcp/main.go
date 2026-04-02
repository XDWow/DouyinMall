package main

import (
	"fmt"
	"log"
	"net/http"

	ordermcp "github.com/XDWow/DouyinMall/backend/internal/order/transport/mcp"
	orderservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	configPath := pflag.String("config", "internal/order/config/dev.yaml", "order mcp config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	var cfg ordermcp.Config
	if err := viper.UnmarshalKey("mcp", &cfg); err != nil {
		log.Fatalf("unmarshal mcp config failed: %v", err)
	}

	serviceName := cfg.Upstream.ServiceName
	if serviceName == "" {
		serviceName = viper.GetString("grpc.server.name")
	}
	addr := cfg.Upstream.DirectAddr
	if addr == "" {
		addr = fmt.Sprintf("127.0.0.1:%d", viper.GetInt("grpc.server.port"))
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":19092"
	}

	orderClient, err := orderservice.NewClient(serviceName, client.WithHostPorts(addr))
	if err != nil {
		log.Fatalf("init order client failed: %v", err)
	}

	srv, err := ordermcp.NewServer(cfg, orderClient)
	if err != nil {
		log.Fatalf("init order mcp server failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", srv)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("order MCP server listening on %s", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, mux); err != nil {
		log.Fatalf("order MCP server failed: %v", err)
	}
}
