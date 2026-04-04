package main

import (
	"fmt"
	"log"
	"net/http"

	productmcp "github.com/XDWow/DouyinMall/backend/internal/product/transport/mcp"
	productservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
	"github.com/cloudwego/kitex/client"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	configPath := pflag.String("config", "internal/product/config/dev.yaml", "product mcp config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	var cfg productmcp.Config
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
		cfg.Server.Addr = ":19095"
	}

	productClient, err := productservice.NewClient(serviceName, client.WithHostPorts(addr))
	if err != nil {
		log.Fatalf("init product client failed: %v", err)
	}

	srv, err := productmcp.NewServer(cfg, productClient)
	if err != nil {
		log.Fatalf("init product mcp server failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", srv)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("product MCP server listening on %s", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, mux); err != nil {
		log.Fatalf("product MCP server failed: %v", err)
	}
}


