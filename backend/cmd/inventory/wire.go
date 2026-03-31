//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/inventory/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer    server.Server
	OrderConsumer *mq.OrderConsumer
}

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer,
		ioc.InitOrderClient,
		cache.NewRedisInventoryCache,
		repository.NewGormInventoryRepository,
		usecase.NewReserveStockUsecase,
		usecase.NewCommitStockUseCase,
		usecase.NewRefundStockUseCase,
		usecase.NewReleaseStockUseCase,
		mq.NewOrderConsumer,
		transportgrpc.NewInventoryHandler,
		ioc.InitGRPCServer,
		wire.Struct(new(App), "*"),
	)
	return nil
}
