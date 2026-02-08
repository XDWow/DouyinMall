//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer    server.Server
	OrderConsumer *mq.OrderConsumer
	//CacheRepair   *job.CacheRepairJob // 缓存修复定时任务
}

func InitApp() *App {
	wire.Build(
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer, // Kafka生产者（死信队列）
		ioc.InitOrderClient,       // 订单服务客户端（同步调用更新状态）

		// Cache
		cache.NewRedisInventoryCache,

		// Repository
		repository.NewGormInventoryRepository,

		// UseCase
		usecase.NewCommitStockUseCase,
		usecase.NewRefundStockUseCase,
		usecase.NewReleaseStockUseCase,

		// Infra - MQ消费者
		mq.NewOrderConsumer,

		// Transport（gRPC handler）
		grpc.NewInventoryHandler,

		// Servers
		ioc.InitGRPCServer, // gRPC 服务器

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
