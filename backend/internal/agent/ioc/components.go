package ioc

import (
	"context"
	"fmt"
	"time"

	agentllm "github.com/XDWow/DouyinMall/backend/internal/agent/components/llm"
	agentprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	agentrag "github.com/XDWow/DouyinMall/backend/internal/agent/components/rag"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentrepository "github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	agentmemory "github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	orchestrator "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator"
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
	Memory          *agentmemory.Manager
	ExactCache      agentcache.ExactCache
	RateLimiter     agentcache.RateLimiter
	CheckpointStore agentcache.CheckpointStore
	Prompts         *agentprompt.Set
	Metrics         *orchestrator.Metrics
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

	chatModel, err := agentllm.NewChatModel(ctx, agentllm.ChatModelConfig{
		BaseURL:     cfg.LLM.BaseURL,
		APIKey:      cfg.LLM.APIKey,
		Model:       cfg.LLM.Model,
		Timeout:     secondsOrDefault(cfg.LLM.TimeoutSeconds, 60*time.Second),
		Temperature: cfg.LLM.Temperature,
		MaxTokens:   cfg.LLM.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("init openai chat model failed: %w", err)
	}

	embedder, err := agentllm.NewEmbedder(ctx, agentllm.EmbeddingConfig{
		BaseURL: cfg.Embedding.BaseURL,
		APIKey:  cfg.Embedding.APIKey,
		Model:   cfg.Embedding.Model,
		Timeout: secondsOrDefault(cfg.Embedding.TimeoutSeconds, 15*time.Second),
	})
	if err != nil {
		return nil, fmt.Errorf("init openai embedder failed: %w", err)
	}

	sessionRepo := agentrepository.NewSessionStore(dao, rdb)
	conversationWindow := cfg.Workflow.ConversationWindow
	if conversationWindow <= 0 {
		conversationWindow = 5
	}

	return &Components{
		Model:     chatModel,
		Embedder:  embedder,
		Retriever: agentrag.NewVectorRetriever(agentrepository.NewKnowledgeStore(dao), embedder, retrieveTopK),
		Registry:  registry,
		// Memory wraps the session repository and enforces the conversation
		// window so the orchestrator doesn't need to know about persistence.
		Memory:          agentmemory.New(sessionRepo, conversationWindow),
		ExactCache:      agentcache.NewRedisExactCache(rdb),
		RateLimiter:     agentcache.NewRedisRateLimiter(rdb),
		CheckpointStore: agentcache.NewRedisCheckpointStore(rdb, secondsOrDefault(cfg.Workflow.CheckpointTTLSeconds, 7*24*time.Hour)),
		Prompts:         agentprompt.NewDefault(),
		Metrics:         orchestrator.NewMetrics("douyinmall_agent"),
	}, nil
}

func secondsOrDefault(raw int, fallback time.Duration) time.Duration {
	if raw <= 0 {
		return fallback
	}
	return time.Duration(raw) * time.Second
}
