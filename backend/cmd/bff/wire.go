//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/bff/handler"
	"github.com/XDWow/DouyinMall/backend/internal/bff/ioc"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		// JWT
		ioc.InitJWTManager,

		// Redis + 限流
		ioc.InitRedis,
		ioc.InitRateLimiter,

		// RPC 客户端
		ioc.InitAgentClient,
		ioc.InitAgentStreamClient,
		ioc.InitUserClient,

		// Handler
		handler.NewAgentHandler,
		handler.NewAuthHandler,

		// HTTP Server
		ioc.InitGinServer,

		// App
		newApp,
	)
	return nil
}

func newApp(svr *ginx.Server) *App {
	return &App{Server: svr}
}
