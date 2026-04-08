package ioc

import (
	"context"
	"fmt"
	"time"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentllm "github.com/XDWow/DouyinMall/backend/internal/agent/infra/llm"
	agentrag "github.com/XDWow/DouyinMall/backend/internal/agent/infra/rag"
	agentrepository "github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestrator "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator"
	agentprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/redis/go-redis/v9"
)

// Components groups the Eino components and runtime-facing capabilities
// used by the agent graph. App bootstrap should construct them once here
// and only pass references into the graph runtime.
type Components struct {
	Model           model.ToolCallingChatModel
	Embedder        embedding.Embedder
	KnowledgeBase   *agentrag.ManagedKnowledgeService
	Skills          *agentskill.Registry
	Registry        *agenttool.Registry
	SessionService  *agentsession.Service
	ExactCache      agentcache.ExactCache
	SemanticCache   agentcache.SemanticCache
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
		return nil, fmt.Errorf("init embedding model failed: %w", err)
	}

	knowledgeBase, err := agentrag.NewManagedKnowledgeService(agentrag.KnowledgeServiceConfig{
		Scheme:            cfg.KnowledgeBase.Scheme,
		Domain:            cfg.KnowledgeBase.Domain,
		ServiceChatPath:   cfg.KnowledgeBase.ServiceChatPath,
		ServiceResourceID: cfg.KnowledgeBase.ServiceResourceID,
		APIKey:            cfg.KnowledgeBase.APIKey,
		Timeout:           secondsOrDefault(cfg.KnowledgeBase.TimeoutSeconds, 60*time.Second),
	})
	if err != nil {
		return nil, fmt.Errorf("init managed knowledge service failed: %w", err)
	}

	var skills *agentskill.Registry
	if cfg.Skill.Enabled {
		skills, err = agentskill.NewRegistry(cfg.Skill.Roots...)
		if err != nil {
			return nil, fmt.Errorf("init skill registry failed: %w", err)
		}
	}

	store := agentcache.NewRedisStore(rdb)
	sessionRepo := agentrepository.NewSessionStore(dao, agentcache.NewRedisSessionCache(store, 24*time.Hour, 10))
	conversationWindow := cfg.Workflow.ConversationWindow
	if conversationWindow <= 0 {
		conversationWindow = 5
	}

	return &Components{
		Model:         chatModel,
		Embedder:      embedder,
		KnowledgeBase: knowledgeBase,
		Skills:        skills,
		Registry:      registry,
		// SessionService wraps the session repository and enforces the
		// conversation window so the orchestrator doesn't need to know persistence details.
		SessionService:  agentsession.NewService(sessionRepo, conversationWindow),
		ExactCache:      agentcache.NewRedisExactCache(store),
		SemanticCache:   agentcache.NewRedisSemanticCache(store),
		RateLimiter:     agentcache.NewRedisRateLimiter(store),
		CheckpointStore: agentcache.NewRedisCheckpointStore(store, secondsOrDefault(cfg.Workflow.CheckpointTTLSeconds, 7*24*time.Hour)),
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
