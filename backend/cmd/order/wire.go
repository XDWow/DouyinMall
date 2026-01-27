//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/order/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/order/job"
	"github.com/XDWow/DouyinMall/backend/internal/order/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
	"github.com/robfig/cron/v3"
)

type App struct {
	Server server.Server
	Cron   *cron.Cron
}

func InitApp() *App {
	wire.Build(
		// 基础设施层
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer,

		// DB层
		db.NewGormTxManager,

		// Cache层
		cache.NewRedisOrderCache,

		// MQ层
		mq.NewSaramaProducer,

		// Repository层
		repository.NewOrderRepository,
		repository.NewOutboxRepository,

		// UseCase层
		usecase.NewCreateOrderUseCase,
		usecase.NewListUserOrderUseCase,
		usecase.NewChangeOrderStatusUseCase,
		usecase.NewBatchCancelOrderUseCase,

		// Job层
		job.NewCheckExpiredJob,
		job.NewOutboxWorkerJob,
		ioc.InitJobs,

		// Transport层（Handler）
		grpc.NewOrderHandler,

		// gRPC Server
		ioc.InitGRPCServer,

		// 组装App
		wire.Struct(new(App), "*"),
	)

	return &App{}
}
