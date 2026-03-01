//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/search/events"
	es "github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/XDWow/DouyinMall/backend/internal/search/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/search/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/search/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitES,
		ioc.InitKafkaClient,
		ioc.InitProductClient,
		ioc.InitLLMClient,
		ioc.InitEmbedder,

		// 仓储
		es.NewProductRepo,
		es.NewMerchantRepo,

		// 用例
		usecase.NewSearchProductsUseCase,
		usecase.NewSearchMerchantsUseCase,
		usecase.NewSuggestUseCase,
		usecase.NewAggregationsUseCase,
		usecase.NewAISearchUseCase,
		usecase.NewSyncUseCase,

		// 传输层
		transportgrpc.NewSearchHandler,
		ioc.InitGRPCServer,

		// Kafka 消费者
		events.NewProductConsumer,
		events.NewMerchantConsumer,

		newApp,
	)
	return nil
}

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
