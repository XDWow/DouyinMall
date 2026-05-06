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
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/order/transport/grpc"
	transporthttp "github.com/XDWow/DouyinMall/backend/internal/order/transport/http"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
	"github.com/robfig/cron/v3"
)

type ConsumerComponent interface {
	Start() error
}

type App struct {
	Server     server.Server
	HTTPServer *ginx.Server
	Cron       *cron.Cron
	Consumers  []ConsumerComponent

	getOrderUC      *usecase.GetOrderUseCase
	listUserOrderUC *usecase.ListUserOrderUseCase
}

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitPaymentClient,
		ioc.InitRocketMQProducerClient,
		ioc.InitOrderStatusProducer,
		ioc.InitOrderMQProducer,
		ioc.InitPaymentStatusConsumerClient,
		ioc.InitRocketMQConsumerOptions,
		db.NewGormTxManager,
		cache.NewRedisOrderCache,
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
		transportgrpc.NewOrderHandler,
		transporthttp.NewHandler,
		mq.NewPaymentStatusConsumer,
		ioc.InitGRPCServer,
		ioc.InitHTTPServer,
		newApp,
	)

	return nil
}

func newApp(
	srv server.Server,
	httpServer *ginx.Server,
	cron *cron.Cron,
	paymentStatusConsumer *mq.PaymentStatusConsumer,
	getOrderUC *usecase.GetOrderUseCase,
	listUserOrderUC *usecase.ListUserOrderUseCase,
) *App {
	return &App{
		Server:          srv,
		HTTPServer:      httpServer,
		Cron:            cron,
		Consumers:       []ConsumerComponent{paymentStatusConsumer},
		getOrderUC:      getOrderUC,
		listUserOrderUC: listUserOrderUC,
	}
}
