package main

import (
	"log"
	"net/http"
	"strings"

	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	aftersaledb "github.com/XDWow/DouyinMall/backend/internal/aftersale/infra/db"
	aftersalerepository "github.com/XDWow/DouyinMall/backend/internal/aftersale/infra/repository"
	aftersaleioc "github.com/XDWow/DouyinMall/backend/internal/aftersale/ioc"
	aftersalegrpc "github.com/XDWow/DouyinMall/backend/internal/aftersale/transport/grpc"
	aftersaleusecase "github.com/XDWow/DouyinMall/backend/internal/aftersale/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	if err := envx.Load(); err != nil {
		log.Fatalf("load .env failed: %v", err)
	}

	configPath := pflag.String("config", "internal/aftersale/config/dev.yaml", "aftersale service config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
	_ = viper.BindEnv("mcp.server.addr", "MCP_ADDR")
	_ = viper.BindEnv("mcp.upstream.direct_addr", "MCP_UPSTREAM_DIRECT_ADDR")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	var cfg aftersaleconfig.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("unmarshal config failed: %v", err)
	}

	db, err := aftersaleioc.InitDB(cfg)
	if err != nil {
		log.Fatalf("open db failed: %v", err)
	}
	if err := aftersaledb.InitTables(db); err != nil {
		log.Fatalf("init aftersale tables failed: %v", err)
	}

	repository := aftersalerepository.NewRequestRepository(db)
	createRequestUC := aftersaleusecase.NewCreateAfterSaleRequestUseCase(repository)
	getRequestUC := aftersaleusecase.NewGetAfterSaleRequestUseCase(repository)
	handler := aftersalegrpc.NewHandler(createRequestUC, getRequestUC)

	svr, err := aftersaleioc.InitGRPCServer(cfg, handler)
	if err != nil {
		log.Fatalf("init aftersale server failed: %v", err)
	}

	log.Printf("aftersale service listening on :%d", cfg.GRPC.Server.Port)
	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- svr.Run()
	}()

	var mcpErr <-chan error
	if strings.TrimSpace(cfg.MCP.Server.Addr) != "" {
		mcpHandler, err := newMCPHandler(cfg)
		if err != nil {
			log.Fatalf("init aftersale MCP failed: %v", err)
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
			log.Printf("aftersale MCP listening on %s", cfg.MCP.Server.Addr)
			mcpErrCh <- http.ListenAndServe(cfg.MCP.Server.Addr, mux)
		}()
	}

	select {
	case err := <-grpcErr:
		if err != nil {
			log.Fatalf("aftersale gRPC server failed: %v", err)
		}
	case err := <-mcpErr:
		if err != nil {
			log.Fatalf("aftersale MCP server failed: %v", err)
		}
	}
}
