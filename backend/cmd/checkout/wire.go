//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/checkout/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/checkout/transport/grpc"
	transporthttp "github.com/XDWow/DouyinMall/backend/internal/checkout/transport/http"
	"github.com/XDWow/DouyinMall/backend/internal/checkout/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/google/wire"
)

type App struct {
	GRPCServer server.Server
	HTTPServer *ginx.Server
}

func InitApp() *App {
	wire.Build(
		// 鍩虹璁炬柦
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
		transporthttp.NewHandler,

		// Server
		ioc.InitGRPCServer,
		ioc.InitHTTPServer,

		// App
		newApp,
	)
	return nil
}

func newApp(grpcServer server.Server, httpServer *ginx.Server) *App {
	return &App{GRPCServer: grpcServer, HTTPServer: httpServer}
}


