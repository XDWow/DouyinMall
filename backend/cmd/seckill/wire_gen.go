package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/ioc"
	transportgrpc "github.com/XDWow/DouyinMall/backend/internal/seckill/transport/grpc"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/usecase"
	"github.com/cloudwego/kitex/server"
)

type App struct {
	GRPCServer          server.Server
	SeckillConsumer     *mq.SeckillConsumer
	OrderStatusConsumer *mq.OrderStatusConsumer
}

func InitApp() *App {
	loggerV1 := ioc.InitLogger()
	gormDB := ioc.InitDB()
	cmdable := ioc.InitRedis()
	redisCache := cache.NewRedisCache(cmdable)
	cache2 := repository.NewCacheRepository(redisCache)
	activityRepository := repository.NewActivityRepository(gormDB)
	requestRepository := repository.NewRequestRepository(gormDB)
	client := ioc.InitKafkaClient()
	syncProducer := ioc.InitKafkaSyncProducer(client)
	producer := mq.NewProducer(syncProducer)
	idGenerator := ioc.InitIDGenerator()
	createActivityUseCase := usecase.NewCreateActivityUseCase(activityRepository, cache2)
	updateActivityStatusUseCase := usecase.NewUpdateActivityStatusUseCase(activityRepository, cache2)
	getActivityUseCase := usecase.NewGetActivityUseCase(activityRepository, cache2)
	submitUseCase := usecase.NewSubmitUseCase(activityRepository, requestRepository, cache2, producer, idGenerator)
	getResultUseCase := usecase.NewGetResultUseCase(requestRepository, cache2)
	handler := transportgrpc.NewHandler(createActivityUseCase, updateActivityStatusUseCase, getActivityUseCase, submitUseCase, getResultUseCase)
	grpcServer := ioc.InitGRPCServer(handler)
	orderClient := ioc.InitOrderClient()
	seckillConsumer := mq.NewSeckillConsumer(client, syncProducer, orderClient, requestRepository, activityRepository, cache2, loggerV1)
	orderStatusConsumer := mq.NewOrderStatusConsumer(client, requestRepository, activityRepository, cache2, loggerV1)
	return &App{
		GRPCServer:          grpcServer,
		SeckillConsumer:     seckillConsumer,
		OrderStatusConsumer: orderStatusConsumer,
	}
}


