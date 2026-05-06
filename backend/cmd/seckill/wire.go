//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/seckill/transport/grpc"
	transporthttp "github.com/XDWow/DouyinMall/backend/internal/seckill/transport/http"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer          server.Server
	HTTPServer          *ginx.Server
	Producer            *mq.Producer
	SeckillConsumer     *mq.SeckillConsumer
	DeadLetterConsumer  *mq.DeadLetterConsumer
	OrderStatusConsumer *mq.OrderStatusConsumer
}

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitRocketMQProducerClient,
		ioc.InitSeckillSimpleConsumer,
		ioc.InitSeckillDeadLetterSimpleConsumer,
		ioc.InitSeckillOrderStatusSimpleConsumer,
		ioc.InitSeckillConsumerOptions,
		ioc.InitSeckillOrderStatusConsumerOptions,
		ioc.InitOrderClient,
		ioc.InitIDGenerator,
		cache.NewRedisCache,
		repository.NewCacheRepository,
		repository.NewActivityRepository,
		repository.NewRequestRepository,
		mq.NewEventProcessor,
		mq.NewProducer,
		usecase.NewCreateActivityUseCase,
		usecase.NewUpdateActivityStatusUseCase,
		usecase.NewGetActivityUseCase,
		usecase.NewSubmitUseCase,
		usecase.NewGetResultUseCase,
		mq.NewSeckillConsumer,
		mq.NewDeadLetterConsumer,
		mq.NewOrderStatusConsumer,
		transportgrpc.NewHandler,
		transporthttp.NewHandler,
		ioc.InitGRPCServer,
		ioc.InitHTTPServer,
		wire.Struct(new(App), "*"),
	)
	return nil
}
