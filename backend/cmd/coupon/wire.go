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
		// 基础设施
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

		// MQ消费者
		mq.NewOrderConsumer,

		// Transport（gRPC handler）
		grpc.NewCouponHandler,

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
