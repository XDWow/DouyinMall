//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/handler"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/knowledge"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/persistence"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/agent/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitLLMClient,
		ioc.InitEmbedder,

		// DAO
		persistence.NewAgentDAO,

		// Cache
		agentcache.NewRedisSessionCache,

		// Repository（组合 Redis + MySQL）
		repository.NewSessionRepo,

		// Milvus Knowledge & Semantic Cache
		// TODO: 补充 ioc.InitMilvusClient 后取消注释
		// knowledge.NewMilvusKnowledgeRepo,
		// knowledge.NewSemanticCache,
		wire.Value(knowledge.MilvusClient(nil)), // 占位，待 Milvus SDK 接入后替换
		knowledge.NewMilvusKnowledgeRepo,
		knowledge.NewSemanticCache,

		// UseCase
		usecase.NewChatUseCase,
		usecase.NewSessionUseCase,

		// Handler
		handler.NewAgentHandler,

		// gRPC Server
		ioc.InitGRPCServer,

		// App
		newApp,
	)
	return nil
}

func newApp(svr server.Server) *App {
	return &App{Server: svr}
}
