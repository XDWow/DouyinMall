package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	productmcp "github.com/XDWow/DouyinMall/backend/internal/product/transport/mcp"
	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	app := InitApp()

	log.Printf("product service starting, grpc port=%d http port=%d",
		viper.GetInt("grpc.server.port"),
		viper.GetInt("http.server.port"))

	if viper.GetBool("canal.enabled") {
		ctx := context.Background()
		for i, p := range app.Producers {
			producer := p
			go func(idx int) {
				if err := producer.Start(ctx); err != nil {
					log.Fatalf("producer %d start failed: %v", idx+1, err)
				}
			}(i)
			log.Printf("producer %d started", i+1)
		}
	} else {
		log.Printf("canal producer disabled by config")
	}

	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- app.Server.Run()
	}()

	httpErr := make(chan error, 1)
	go func() {
		httpErr <- app.HTTPServer.Start()
	}()

	var mcpCfg productmcp.Config
	mcpOK := viper.UnmarshalKey("mcp", &mcpCfg) == nil && strings.TrimSpace(mcpCfg.Server.Addr) != ""
	if mcpOK {
		mcpHandler, err := newMCPHandler(mcpCfg)
		if err != nil {
			log.Fatalf("init product MCP failed: %v", err)
		}
		mux := http.NewServeMux()
		mux.Handle("/mcp", mcpHandler)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		go func() {
			log.Printf("product MCP listening on %s", mcpCfg.Server.Addr)
			if err := http.ListenAndServe(mcpCfg.Server.Addr, mux); err != nil {
				log.Fatalf("product MCP server exited: %v", err)
			}
		}()
	}

	select {
	case err := <-grpcErr:
		if err != nil {
			log.Fatalf("grpc server run error: %v", err)
		}
	case err := <-httpErr:
		if err != nil {
			log.Fatalf("http server run error: %v", err)
		}
	}
}

func initViperWatch() {
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	cfile := pflag.String("config", "internal/product/config/dev.yaml", "config file path")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %w", err))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = viper.BindEnv("canal.enabled", "CANAL_ENABLED")
	_ = viper.BindEnv("canal.mysql.password", "CANAL_MYSQL_PASSWORD")
	_ = viper.BindEnv("mcp.server.addr", "MCP_ADDR")
	_ = viper.BindEnv("mcp.upstream.direct_addr", "MCP_UPSTREAM_DIRECT_ADDR")
}
