//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer    server.Server
	OrderConsumer *mq.OrderConsumer
}

func InitApp() *App {
	wire.Build(
		// 鍩虹璁炬柦
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitKafkaClient,

		// Repository
		repository.NewCouponRepository,
		repository.NewCouponTemplateRepository,
		repository.NewCouponOperationRepository,

		// UseCase
		usecase.NewListUserCouponsUseCase,
		usecase.NewEvaluateOrderCouponsUseCase,
		usecase.NewReserveCouponUseCase,
		usecase.NewCommitCouponUseCase,
		usecase.NewReleaseCouponUseCase,
		usecase.NewRefundCouponUseCase,
		usecase.NewIssueCouponUseCase,

		// MQ娑堣垂鑰?		mq.NewOrderConsumer,

		// Transport锛坓RPC handler锛?		grpc.NewCouponHandler,

		// Servers
		ioc.InitGRPCServer,

		// App
		newApp,
	)
	return nil
}

func newApp(
	grpcServer server.Server,
	orderConsumer *mq.OrderConsumer,
) *App {
	return &App{
		GRPCServer:    grpcServer,
		OrderConsumer: orderConsumer,
	}
}


