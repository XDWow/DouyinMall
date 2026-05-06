//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/product/handler"
	"github.com/XDWow/DouyinMall/backend/internal/product/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/product/producer"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/cache"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/product/service"
	transporthttp "github.com/XDWow/DouyinMall/backend/internal/product/transport/http"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer,
		dao.NewGORMProductDao,
		cache.NewRedisProductCache,
		repo.NewCachedProductRepo,
		service.NewProductService,
		handler.NewProductHandler,
		transporthttp.NewHandler,
		ioc.InitCanalProducer,
		ioc.InitGRPCServer,
		ioc.InitHTTPServer,
		newApp,
	)
	return nil
}

func newApp(svr server.Server, httpServer *ginx.Server, p producer.Producer) *App {
	return &App{
		Server:     svr,
		HTTPServer: httpServer,
		Producers: []ProducerComponent{
			&producerWrapper{Producer: p},
		},
	}
}
