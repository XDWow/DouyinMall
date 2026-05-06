//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/payment/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/payment/job"
	"github.com/XDWow/DouyinMall/backend/internal/payment/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
	"github.com/robfig/cron/v3"
)

type App struct {
	GRPCServer server.Server
	HTTPServer interface {
		Start() error
	}
	Cron *cron.Cron
}

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRocketMQProducerClient,
		ioc.InitPaymentStatusProducer,
		ioc.InitPaymentMQProducer,
		ioc.InitNativePayService,
		ioc.InitPaymentProvider,
		ioc.InitAlipayClient,
		ioc.InitWechatNotifyHandler,

		repository.NewPaymentRepository,
		repository.NewPaymentOutboxRepository,
		db.NewGormTxManager,

		usecase.NewPayCallbackUC,
		ioc.InitNativePrePaymentUC,
		ioc.InitSyncPaymentOrderUC,
		usecase.NewGetPaymentUC,
		usecase.NewConfirmPaymentUC,

		grpc.NewPaymentHandler,

		job.NewSyncPaymentOrderJob,
		job.NewPaymentOutboxWorkerJob,
		ioc.InitJobs,

		ioc.InitGRPCServer,
		ioc.InitHTTPServer,
		newApp,
	)
	return nil
}

func newApp(
	grpcServer server.Server,
	httpServer *ginx.Server,
	cron *cron.Cron,
) *App {
	return &App{
		GRPCServer: grpcServer,
		HTTPServer: httpServer,
		Cron:       cron,
	}
}
