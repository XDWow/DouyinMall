//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/search/events"
	es "github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/XDWow/DouyinMall/backend/internal/search/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/search/transport/grpc"
	transporthttp "github.com/XDWow/DouyinMall/backend/internal/search/transport/http"
	"github.com/XDWow/DouyinMall/backend/internal/search/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitES,
		ioc.InitKafkaClient,
		ioc.InitProductClient,
		ioc.InitLLMClient,
		ioc.InitEmbedder,
		es.NewProductRepo,
		es.NewMerchantRepo,
		usecase.NewSearchProductsUseCase,
		usecase.NewSearchMerchantsUseCase,
		usecase.NewSuggestUseCase,
		usecase.NewAggregationsUseCase,
		usecase.NewAISearchUseCase,
		usecase.NewSyncUseCase,
		transportgrpc.NewSearchHandler,
		transporthttp.NewHandler,
		ioc.InitGRPCServer,
		ioc.InitHTTPServer,
		events.NewProductConsumer,
		events.NewMerchantConsumer,
		newApp,
	)
	return nil
}

func newApp(
	svr server.Server,
	httpServer *ginx.Server,
	productConsumer *events.ProductConsumer,
	merchantConsumer *events.MerchantConsumer,
) *App {
	return &App{
		Server:     svr,
		HTTPServer: httpServer,
		Consumers: []saramax.Consumer{
			productConsumer,
			merchantConsumer,
		},
	}
}
