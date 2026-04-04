//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/cart/handler"
	"github.com/XDWow/DouyinMall/backend/internal/cart/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/cart/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository/cache"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository/dao"
	"github.com/XDWow/DouyinMall/backend/internal/cart/service"
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

		// DAO
		dao.NewGORMCartDAO,

		// Cache
		cache.NewRedisCache,

		// Repository
		repository.NewCachedCartRepository,

		// Service
		service.NewCartService,

		// handler
		handler.NewCartHandler,

		// MQ Consumer
		mq.NewOrderConsumer,

		// gRPC Server
		ioc.InitGRPCServer,

		// App
		newApp,
	)
	return nil
}

// newApp 缁勮 App
func newApp(svr server.Server, consumer *mq.OrderConsumer) *App {
	return &App{Server: svr, OrderConsumer: consumer}
}


