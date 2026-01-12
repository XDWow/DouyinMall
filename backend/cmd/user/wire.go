//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/user/handler"
	"github.com/XDWow/DouyinMall/backend/internal/user/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/cache"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/dao"
	"github.com/XDWow/DouyinMall/backend/internal/user/service"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

// InitApp 初始化整个应用
func InitApp() server.Server {
	wire.Build(
		// 基础设施层（ioc 包提供）
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,

		// DAO 层
		dao.NewGORMUserDAO,

		// Cache 层
		cache.NewRedisUserCache,

		// Repository 层
		repo.NewUserRepository,

		// Service 层
		service.NewUserService,

		// Handler 层
		handler.NewUserServiceServer,

		// gRPC Server
		ioc.InitGRPCServer,
	)
	return nil
}
