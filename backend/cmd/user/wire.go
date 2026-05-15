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

func InitApp() server.Server {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		dao.NewGORMUserDAO,
		cache.NewRedisUserCache,
		repo.NewUserRepository,
		service.NewUserService,
		handler.NewUserServiceServer,
		ioc.InitGRPCServer,
	)
	return nil
}
