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
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		// 鍩虹璁炬柦
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer,

		// DAO
		dao.NewGORMProductDao,

		// Cache
		cache.NewRedisProductCache,

		// Repository
		repo.NewCachedProductRepo,

		// Service
		service.NewProductService,

		// handler
		handler.NewProductHandler,

		// Producer
		ioc.InitCanalProducer,

		// gRPC Server
		ioc.InitGRPCServer,

		// App
		newApp,
	)
	return nil
}

// newApp 缁勮 App
func newApp(svr server.Server, p producer.Producer) *App {
	return &App{
		Server: svr,
		Producers: []ProducerComponent{
			&producerWrapper{Producer: p},
		},
	}
}


