package main

import (
	"fmt"
	"log"
	"net/http"

	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	aftersalemcp "github.com/XDWow/DouyinMall/backend/internal/aftersale/transport/mcp"
	aftersaleservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/aftersale/v1/aftersaleservice"
	"github.com/cloudwego/kitex/client"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	configPath := pflag.String("config", "internal/aftersale/config/dev.yaml", "aftersale mcp config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	var cfg aftersaleconfig.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("unmarshal config failed: %v", err)
	}

	serviceName := cfg.MCP.Upstream.ServiceName
	if serviceName == "" {
		serviceName = cfg.GRPC.Server.Name
	}
	if serviceName == "" {
		serviceName = "aftersale.service"
	}

	addr := cfg.MCP.Upstream.DirectAddr
	if addr == "" {
		addr = fmt.Sprintf("127.0.0.1:%d", cfg.GRPC.Server.Port)
	}

	aftersaleClient, err := aftersaleservice.NewClient(serviceName, client.WithHostPorts(addr))
	if err != nil {
		log.Fatalf("init aftersale client failed: %v", err)
	}

	srv, err := aftersalemcp.NewServer(cfg.MCP, aftersaleClient)
	if err != nil {
		log.Fatalf("init aftersale mcp server failed: %v", err)
	}

	serverAddr := cfg.MCP.Server.Addr
	if serverAddr == "" {
		serverAddr = ":19097"
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", srv)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("aftersale MCP server listening on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("aftersale MCP server failed: %v", err)
	}
}
