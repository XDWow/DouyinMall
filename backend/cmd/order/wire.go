//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/queue"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/order/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/order/job"
	"github.com/XDWow/DouyinMall/backend/internal/order/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
	"github.com/robfig/cron/v3"
)

type ConsumerComponent interface {
	Start() error
}

type App struct {
	Server    server.Server
	Cron      *cron.Cron
	Consumers []ConsumerComponent
}

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitPaymentClient,
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer,
		db.NewGormTxManager,
		cache.NewRedisOrderCache,
		mq.NewSaramaProducer,
		queue.NewOrderDelayQueue,
		repository.NewOrderRepository,
		repository.NewOutboxRepository,
		usecase.NewCreateOrderUseCase,
		usecase.NewGetOrderUseCase,
		usecase.NewListUserOrderUseCase,
		usecase.NewChangeOrderStatusUseCase,
		usecase.NewBatchCancelOrderUseCase,
		job.NewDispatchOrderTimeoutJob,
		job.NewCheckExpiredJob,
		job.NewOutboxWorkerJob,
		ioc.InitJobs,
		grpc.NewOrderHandler,
		ioc.InitGRPCServer,
		newApp,
	)

	return &App{}
}

func newApp(server server.Server, cron *cron.Cron) *App {
	return &App{
		Server:    server,
		Cron:      cron,
		Consumers: nil,
	}
}
