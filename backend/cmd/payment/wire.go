//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/payment/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/payment/job"
	"github.com/XDWow/DouyinMall/backend/internal/payment/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer server.Server // gRPC 服务（其他微服务调用）
	HTTPServer interface {   // HTTP 服务（微信回调）
		Start() error
	}
	Job job.Job // 定时任务
}

func InitApp() *App {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitOrderClient,
		ioc.InitWechatNativeApiService,
		ioc.InitWechatNativeService,

		// Repository
		repository.NewPaymentRepository,

		// Use Cases
		usecase.NewPayCallbackUC,
		ioc.InitNativePrePaymentUC,
		ioc.InitSyncWechatOrderUC,
		usecase.NewGetPaymentUC,

		// Transport
		grpc.NewPaymentHandler, // gRPC handler

		// Job
		job.NewSyncWechatOrderJob,

		// Servers
		ioc.InitGRPCServer, // gRPC 服务器
		ioc.InitHTTPServer, // HTTP 服务器

		// App
		newApp,
	)
	return nil
}

func newApp(
	grpcServer server.Server,
	httpServer *ginx.Server,
	syncJob *job.SyncWechatOrderJob,
) *App {
	return &App{
		GRPCServer: grpcServer,
		HTTPServer: httpServer,
		Job:        syncJob,
	}
}
