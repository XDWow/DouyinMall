package ioc

import (
	"context"
	"fmt"
	"time"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
	customergraph "github.com/XDWow/DouyinMall/backend/internal/agent/graph"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentrepository "github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	agentrag "github.com/XDWow/DouyinMall/backend/internal/agent/rag"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
	openaiembedder "github.com/cloudwego/eino-ext/components/embedding/openai"
	openaichat "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/redis/go-redis/v9"
)

// Components groups the Eino components and runtime-facing capabilities
// used by the agent graph. App bootstrap should construct them once here
// and only pass references into the graph runtime.
type Components struct {
	Model           model.ToolCallingChatModel
	Embedder        embedding.Embedder
	Retriever       einoretriever.Retriever
	Registry        *agenttool.Registry
	SessionStore    memory.Store
	ExactCache      agentcache.ExactCache
	RateLimiter     agentcache.RateLimiter
	CheckpointStore agentcache.CheckpointStore
	Prompts         *customergraph.PromptSet
	Metrics         *customergraph.Metrics
}

func InitComponents(
	ctx context.Context,
	cfg agentconfig.Config,
	dao *agentrepository.DAO,
	rdb *redis.Client,
	retrieveTopK int,
) (*Components, error) {
	registry, err := agenttool.NewMCPRegistry(ctx, cfg.MCP.Servers)
	if err != nil {
		return nil, fmt.Errorf("init tool registry failed: %w", err)
	}

	chatModel, err := openaichat.NewChatModel(ctx, &openaichat.ChatModelConfig{
		BaseURL:     cfg.LLM.BaseURL,
		APIKey:      cfg.LLM.APIKey,
		Model:       cfg.LLM.Model,
		Timeout:     secondsOrDefault(cfg.LLM.TimeoutSeconds, 60*time.Second),
		Temperature: float32Ptr(cfg.LLM.Temperature),
		MaxTokens:   intPtr(cfg.LLM.MaxTokens),
	})
	if err != nil {
		return nil, fmt.Errorf("init openai chat model failed: %w", err)
	}

	embedder, err := openaiembedder.NewEmbedder(ctx, &openaiembedder.EmbeddingConfig{
		BaseURL: cfg.Embedding.BaseURL,
		APIKey:  cfg.Embedding.APIKey,
		Model:   cfg.Embedding.Model,
		Timeout: secondsOrDefault(cfg.Embedding.TimeoutSeconds, 15*time.Second),
	})
	if err != nil {
		return nil, fmt.Errorf("init openai embedder failed: %w", err)
	}

	return &Components{
		Model:           chatModel,
		Embedder:        embedder,
		Retriever:       agentrag.NewVectorRetriever(agentrepository.NewKnowledgeStore(dao), embedder, retrieveTopK),
		Registry:        registry,
		SessionStore:    agentrepository.NewSessionStore(dao, rdb),
		ExactCache:      agentcache.NewRedisExactCache(rdb),
		RateLimiter:     agentcache.NewRedisRateLimiter(rdb),
		CheckpointStore: agentcache.NewRedisCheckpointStore(rdb, secondsOrDefault(cfg.Workflow.CheckpointTTLSeconds, 7*24*time.Hour)),
		Prompts:         customergraph.NewDefaultPrompts(),
		Metrics:         customergraph.NewMetrics("douyinmall_agent"),
	}, nil
}

func secondsOrDefault(raw int, fallback time.Duration) time.Duration {
	if raw <= 0 {
		return fallback
	}
	return time.Duration(raw) * time.Second
}

func intPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func float32Ptr(value float32) *float32 {
	if value == 0 {
		return nil
	}
	return &value
}
