package main

import (
	"log"

	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	aftersaledb "github.com/XDWow/DouyinMall/backend/internal/aftersale/infra/db"
	aftersalerepository "github.com/XDWow/DouyinMall/backend/internal/aftersale/infra/repository"
	aftersaleioc "github.com/XDWow/DouyinMall/backend/internal/aftersale/ioc"
	aftersalegrpc "github.com/XDWow/DouyinMall/backend/internal/aftersale/transport/grpc"
	aftersaleusecase "github.com/XDWow/DouyinMall/backend/internal/aftersale/usecase"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	configPath := pflag.String("config", "internal/aftersale/config/dev.yaml", "aftersale service config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
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
	if err := svr.Run(); err != nil {
		log.Fatalf("aftersale service failed: %v", err)
	}
}
