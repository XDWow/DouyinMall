package main

import (
	"fmt"
	"log"
	"net/http"

	inventorymcp "github.com/XDWow/DouyinMall/backend/internal/inventory/transport/mcp"
	inventoryservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
	"github.com/cloudwego/kitex/client"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	configPath := pflag.String("config", "internal/inventory/config/dev.yaml", "inventory mcp config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	var cfg inventorymcp.Config
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
		cfg.Server.Addr = ":19096"
	}

	inventoryClient, err := inventoryservice.NewClient(serviceName, client.WithHostPorts(addr))
	if err != nil {
		log.Fatalf("init inventory client failed: %v", err)
	}

	srv, err := inventorymcp.NewServer(cfg, inventoryClient)
	if err != nil {
		log.Fatalf("init inventory mcp server failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", srv)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("inventory MCP server listening on %s", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, mux); err != nil {
		log.Fatalf("inventory MCP server failed: %v", err)
	}
}


