package ioc

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentmq "github.com/XDWow/DouyinMall/backend/internal/agent/infra/mq"
	agentrag "github.com/XDWow/DouyinMall/backend/internal/agent/infra/rag"
	agentrepository "github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestrator "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/compose"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	pkglogger "github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// Components groups the Eino components and runtime-facing capabilities
// used by the agent graph. App bootstrap should construct them once here
// and only pass references into the graph runtime.
type Components struct {
	LLMs            *pkgai.EinoRouter
	Embedder        embedding.Embedder
	KnowledgeBase   agentrag.Searcher
	Skills          *agentskill.Registry
	Registry        *agenttool.Registry
	SessionService  *agentsession.Service
	ExactCache      agentcache.ExactCache
	SemanticCache   agentcache.SemanticCache
	RateLimiter     agentcache.RateLimiter
	CheckpointStore compose.CheckPointStore
	Metrics         *orchestrator.Metrics
}

func InitComponents(
	ctx context.Context,
	cfg agentconfig.Config,
	db *gorm.DB,
	rdb *redis.Client,
) (*Components, error) {
	var skills *agentskill.Registry
	if cfg.Skill.Enabled {
		var err error
		skills, err = agentskill.NewRegistry(cfg.Skill.Roots...)
		if err != nil {
			return nil, fmt.Errorf("init skill registry failed: %w", err)
		}
	}

	registry, err := agenttool.NewMCPRegistry(ctx, cfg.MCP.Servers, skills)
	if err != nil {
		return nil, fmt.Errorf("init tool registry failed: %w", err)
	}

	weakModel, err := pkgai.NewEinoChatModel(ctx, pkgai.EinoChatModelConfig{
		Provider:    cfg.LLM.Weak.Provider,
		BaseURL:     cfg.LLM.Weak.BaseURL,
		APIKey:      cfg.LLM.Weak.APIKey,
		Model:       cfg.LLM.Weak.Model,
		Timeout:     secondsOrDefault(cfg.LLM.Weak.TimeoutSeconds, 60*time.Second),
		Temperature: cfg.LLM.Weak.Temperature,
		MaxTokens:   cfg.LLM.Weak.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("init weak chat model failed: %w", err)
	}

	strongModel := weakModel
	if strings.TrimSpace(cfg.LLM.Strong.Model) != "" {
		strongModel, err = pkgai.NewEinoChatModel(ctx, pkgai.EinoChatModelConfig{
			Provider:    cfg.LLM.Strong.Provider,
			BaseURL:     cfg.LLM.Strong.BaseURL,
			APIKey:      cfg.LLM.Strong.APIKey,
			Model:       cfg.LLM.Strong.Model,
			Timeout:     secondsOrDefault(cfg.LLM.Strong.TimeoutSeconds, 60*time.Second),
			Temperature: cfg.LLM.Strong.Temperature,
			MaxTokens:   cfg.LLM.Strong.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("init strong chat model failed: %w", err)
		}
	}

	embedder, err := pkgai.NewEinoEmbedder(ctx, pkgai.EinoEmbeddingConfig{
		Provider: cfg.Embedding.Provider,
		BaseURL:  cfg.Embedding.BaseURL,
		APIKey:   cfg.Embedding.APIKey,
		Model:    cfg.Embedding.Model,
		Timeout:  secondsOrDefault(cfg.Embedding.TimeoutSeconds, 15*time.Second),
	})
	if err != nil {
		return nil, fmt.Errorf("init embedding model failed: %w", err)
	}

	knowledgeBase, err := agentrag.NewLocalQdrantKnowledgeService(ctx, agentrag.LocalQdrantConfig{
		Host:            cfg.KnowledgeBase.Qdrant.Host,
		Port:            cfg.KnowledgeBase.Qdrant.Port,
		APIKey:          cfg.KnowledgeBase.Qdrant.APIKey,
		Collection:      cfg.KnowledgeBase.Qdrant.Collection,
		UseTLS:          cfg.KnowledgeBase.Qdrant.UseTLS,
		DefaultTopK:     cfg.Workflow.RetrieveTopK,
		DefaultMinScore: choosePositiveFloat(cfg.KnowledgeBase.Qdrant.ScoreThreshold, cfg.Workflow.RetrieveMinScore),
		Embedder:        embedder,
	})
	if err != nil {
		return nil, fmt.Errorf("init qdrant knowledge service failed: %w", err)
	}

	store := agentcache.NewRedisStore(rdb)
	sessionCache := agentcache.NewRedisSessionCache(store, 24*time.Hour, 10)

	var roundPublisher agentrepository.SessionRoundAsyncPublisher
	if cfg.Kafka.Enabled {
		if len(cfg.Kafka.Brokers) == 0 {
			return nil, fmt.Errorf("kafka.enabled is true but kafka.brokers is empty")
		}
		kafkaClient, err := NewKafkaClient(cfg.Kafka)
		if err != nil {
			return nil, fmt.Errorf("init kafka client failed: %w", err)
		}
		producer, err := NewKafkaSyncProducer(kafkaClient)
		if err != nil {
			_ = kafkaClient.Close()
			return nil, fmt.Errorf("init kafka sync producer failed: %w", err)
		}
		topic := strings.TrimSpace(cfg.Kafka.TopicSessionRound)
		group := strings.TrimSpace(cfg.Kafka.ConsumerGroup)
		roundPublisher = agentmq.NewSessionRoundProducer(producer, topic)

		cons := agentmq.NewSessionRoundConsumer(
			kafkaClient,
			db,
			pkglogger.L(),
			topic,
			group,
			cfg.Kafka.SessionRoundBatchSize,
		)
		if err := cons.Start(); err != nil {
			_ = producer.Close()
			_ = kafkaClient.Close()
			return nil, fmt.Errorf("start session round kafka consumer failed: %w", err)
		}
	}

	sessionRepo := agentrepository.NewSessionRepository(db, sessionCache, roundPublisher)
	conversationWindow := cfg.Workflow.ConversationWindow
	if conversationWindow <= 0 {
		conversationWindow = 5
	}

	return &Components{
		LLMs:          pkgai.NewEinoRouter(weakModel, strongModel, cfg.LLM.DowngradeOne, cfg.LLM.RetryTimes),
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
		Metrics:         orchestrator.NewMetrics("douyinmall_agent"),
	}, nil
}

func secondsOrDefault(raw int, fallback time.Duration) time.Duration {
	if raw <= 0 {
		return fallback
	}
	return time.Duration(raw) * time.Second
}

func choosePositiveFloat(primary, fallback float64) float64 {
	if primary > 0 {
		return primary
	}
	return fallback
}
