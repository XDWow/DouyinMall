//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/cart/handler"
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
		// 基础设施
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,

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

		// gRPC Server
		ioc.InitGRPCServer,

		// App
		newApp,
	)
	return nil
}

// newApp 组装 App
func newApp(svr server.Server) *App {
	return &App{Server: svr}
}
