//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/job"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

type App struct {
	GRPCServer    server.Server       // gRPC 服务（其他微服务调用）
	OrderConsumer *mq.OrderConsumer   // Kafka 消费者（订单状态变更）
	CacheRepair   *job.CacheRepairJob // 缓存修复定时任务
}

func InitApp() *App {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitKafkaClient,

		// Cache
		cache.NewRedisInventoryCache,

		// Repository
		repository.NewGormInventoryRepository,
		wire.Bind(new(domain.InventoryRepository), new(*repository.GormInventoryRepository)),

		// UseCase（细粒度业务逻辑）
		usecase.NewCommitStockUseCase,
		usecase.NewReleaseStockUseCase,
		usecase.NewRefundStockUseCase,

		// Infra - MQ消费者
		mq.NewOrderConsumer,

		// Job - 定时任务
		job.NewCacheRepairJob,

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
	cacheRepair *job.CacheRepairJob,
) *App {
	return &App{
		GRPCServer:    grpcServer,
		OrderConsumer: orderConsumer,
		CacheRepair:   cacheRepair,
	}
}
