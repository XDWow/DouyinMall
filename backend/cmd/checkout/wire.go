//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/checkout/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/checkout/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/checkout/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer server.Server
}

func InitApp() *App {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitIDGenerator,

		// RPC Clients
		ioc.InitProductClient,
		ioc.InitInventoryClient,
		ioc.InitCouponClient,
		ioc.InitOrderClient,
		ioc.InitPaymentClient,

		// Use Cases
		usecase.NewPreviewOrderUseCase,
		usecase.NewPlaceOrderUseCase,
		usecase.NewPayOrderUseCase,

		// Transport
		transportgrpc.NewCheckoutHandler,

		// Server
		ioc.InitGRPCServer,

		// App
		newApp,
	)
	return nil
}

func newApp(grpcServer server.Server) *App {
	return &App{GRPCServer: grpcServer}
}
