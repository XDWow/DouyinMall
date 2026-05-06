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
	transporthttp "github.com/XDWow/DouyinMall/backend/internal/cart/transport/http"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitRocketMQOrderStatusConsumer,
		ioc.InitRocketMQConsumerOptions,
		dao.NewGORMCartDAO,
		cache.NewRedisCache,
		repository.NewCachedCartRepository,
		service.NewCartService,
		handler.NewCartHandler,
		transporthttp.NewHandler,
		mq.NewOrderConsumer,
		ioc.InitGRPCServer,
		ioc.InitHTTPServer,
		newApp,
	)
	return nil
}

func newApp(svr server.Server, httpServer *ginx.Server, consumer *mq.OrderConsumer) *App {
	return &App{Server: svr, HTTPServer: httpServer, OrderConsumer: consumer}
}
