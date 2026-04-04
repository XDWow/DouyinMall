package main

import (
	"fmt"
	"log"
	"net/http"

	cartmcp "github.com/XDWow/DouyinMall/backend/internal/cart/transport/mcp"
	cartservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1/cartservice"
	"github.com/cloudwego/kitex/client"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	configPath := pflag.String("config", "internal/cart/config/dev.yaml", "cart mcp config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	var cfg cartmcp.Config
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
		cfg.Server.Addr = ":19094"
	}

	cartClient, err := cartservice.NewClient(serviceName, client.WithHostPorts(addr))
	if err != nil {
		log.Fatalf("init cart client failed: %v", err)
	}

	srv, err := cartmcp.NewServer(cfg, cartClient)
	if err != nil {
		log.Fatalf("init cart mcp server failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", srv)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("cart MCP server listening on %s", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, mux); err != nil {
		log.Fatalf("cart MCP server failed: %v", err)
	}
}


