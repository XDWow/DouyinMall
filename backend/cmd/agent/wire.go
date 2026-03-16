//go:build wireinject

package main

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/handler"
	infraai "github.com/XDWow/DouyinMall/backend/internal/agent/infra/ai"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/agent/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/agent/knowledge"
	"github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/cloudwego/kitex/server"
	"github.com/google/wire"
)

func InitApp() *App {
	wire.Build(
		// 基础设施
		ioc.InitLogger,
		ioc.InitDB,
		ioc.InitRedis,
		usecase.NewPipelineMetrics,
		ioc.InitFallbackLLMClient,
		// FallbackLLMClient 实现 ai.LLMClient 接口
		wire.Bind(new(ai.LLMClient), new(*infraai.FallbackLLMClient)),
		ioc.InitEmbedder,
		ioc.InitReranker,
		ioc.InitReranker,

		// Kafka
		ioc.InitKafkaClient,
		ioc.InitKafkaSyncProducer,
		mq.NewMessageProducer,
		mq.NewMessageConsumer,

		// Redis 底层（纯 CRUD，不含业务语义）
		// key 构造、序列化、Lua 脚本全部在 repo 层完成
		agentcache.NewAgentRedis,
		ioc.InitSystemRateLimiter,
		ioc.InitUserRateLimiter,

		// Repository（组合 Redis + Kafka + MySQL）
		repository.NewSessionRepo,

		// Milvus Knowledge & Semantic Cache
		// ioc.InitMilvusClient 返回 sdkclient.Client，和 InitRedis 返回 redis.Cmdable 完全对称
		ioc.InitMilvusClient,
		repository.NewMilvusKnowledgeRepo,
		repository.NewSemanticCache,

		// MCP 工具客户端（连接 MCP Tool Server，nil 表示降级纯 RAG）
		ioc.InitMCPClient,

		// UseCase
		usecase.NewAIService,
		usecase.NewChatUseCase,
		usecase.NewSessionUseCase,

		// Handler
		handler.NewAgentHandler,

		// gRPC Server
		ioc.InitGRPCServer,

		// 知识库 Indexer
		knowledge.NewIndexer,

		// App
		newApp,
	)
	return nil
}

func newApp(svr server.Server, consumer *mq.MessageConsumer, indexer *knowledge.Indexer) *App {
	return &App{Server: svr, Consumer: consumer, Indexer: indexer}
}
