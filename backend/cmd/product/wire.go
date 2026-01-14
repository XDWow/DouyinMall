//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/product/handler"
	"github.com/XDWow/DouyinMall/backend/internal/product/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/cache"
	"github.com/XDWow/DouyinMall/backend/internal/product/service"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() server.Server {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,

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

		// gRPC Server
		ioc.InitGRPCServer,
	)
	return nil
}
