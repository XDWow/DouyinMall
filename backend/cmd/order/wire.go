//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/delay_queue"
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

type ConsumerComponent interface {
	Start() error
}

type App struct {
	Server    server.Server
	Cron      *cron.Cron
	Consumers []ConsumerComponent

	getOrderUC      *usecase.GetOrderUseCase
	listUserOrderUC *usecase.ListUserOrderUseCase
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
		delay_queue.NewOrderDelayQueue,
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
		wire.Bind(new(job.BatchCancelOrderExecutor), new(*usecase.BatchCancelOrderUseCase)),
	)

	return nil
}

func newApp(
	srv server.Server,
	cron *cron.Cron,
	getOrderUC *usecase.GetOrderUseCase,
	listUserOrderUC *usecase.ListUserOrderUseCase,
) *App {
	return &App{
		Server:          srv,
		Cron:            cron,
		Consumers:       nil,
		getOrderUC:      getOrderUC,
		listUserOrderUC: listUserOrderUC,
	}
}
