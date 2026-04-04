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

// InitApp 鍒濆鍖栨暣涓簲鐢?
func InitApp() server.Server {
	wire.Build(
		// 鍩虹璁炬柦灞傦紙ioc 鍖呮彁渚涳級
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,

		// DAO 灞?
		dao.NewGORMUserDAO,

		// Cache 灞?
		cache.NewRedisUserCache,

		// Repository 灞?
		repo.NewUserRepository,

		// Service 灞?
		service.NewUserService,

		// Handler 灞?
		handler.NewUserServiceServer,

		// gRPC Server
		ioc.InitGRPCServer,
	)
	return nil
}


