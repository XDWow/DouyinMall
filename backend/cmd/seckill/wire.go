//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/seckill/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer          server.Server
	SeckillConsumer     *mq.SeckillConsumer
	OrderStatusConsumer *mq.OrderStatusConsumer
}

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer,
		ioc.InitOrderClient,
		ioc.InitIDGenerator,
		cache.NewRedisCache,
		repository.NewCacheRepository,
		repository.NewActivityRepository,
		repository.NewRequestRepository,
		mq.NewProducer,
		usecase.NewCreateActivityUseCase,
		usecase.NewUpdateActivityStatusUseCase,
		usecase.NewGetActivityUseCase,
		usecase.NewSubmitUseCase,
		usecase.NewGetResultUseCase,
		mq.NewSeckillConsumer,
		mq.NewOrderStatusConsumer,
		transportgrpc.NewHandler,
		ioc.InitGRPCServer,
		wire.Struct(new(App), "*"),
	)
	return nil
}


