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
		ioc.InitJWTManager,
		ioc.InitRedis,
		ioc.InitRateLimiter,
		ioc.InitAgentClient,
		ioc.InitAgentStreamClient,
		ioc.InitUserClient,
		ioc.InitCheckoutClient,
		ioc.InitSeckillClient,
		ioc.InitOrderClient,
		ioc.InitProductClient,
		ioc.InitInventoryClient,
		handler.NewAgentHandler,
		handler.NewAuthHandler,
		handler.NewTradeHandler,
		ioc.InitGinServer,
		newApp,
	)
	return nil
}

func newApp(svr *ginx.Server) *App {
	return &App{Server: svr}
}


