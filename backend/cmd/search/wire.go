//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/search/events"
	"github.com/XDWow/DouyinMall/backend/internal/search/handler"
	"github.com/XDWow/DouyinMall/backend/internal/search/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/search/repo"
	"github.com/XDWow/DouyinMall/backend/internal/search/service"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

// InitApp 初始化整个应用
func InitApp() *App {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitES,
		ioc.InitKafkaClient,
		
		// RPC 客户端
		ioc.InitProductClient,

		// Repository
		repo.NewProductRepo,
		repo.NewMerchantRepo,

		// Service
		service.NewSearchService,
		service.NewSyncService,

		// Handler
		handler.NewSearchHandler,

		// Consumers
		events.NewProductConsumer,
		events.NewMerchantConsumer,

		// gRPC Server
		ioc.InitGRPCServer,

		// App
		newApp,
	)
	return nil
}

// newApp 组装 App
func newApp(
	svr server.Server,
	productConsumer *events.ProductConsumer,
	merchantConsumer *events.MerchantConsumer,
) *App {
	return &App{
		Server: svr,
		Consumers: []saramax.Consumer{
			productConsumer,
			merchantConsumer,
		},
	}
}
