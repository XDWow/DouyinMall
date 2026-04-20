package orchestrator

import (
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/trace"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	rag "github.com/XDWow/DouyinMall/backend/internal/agent/infra/rag"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

func init() {
	schema.RegisterName[*domain.State]("agent_state_v3")
	schema.RegisterName[*domain.Session]("agent_domain_session_v2")
	schema.RegisterName[*domain.ChatResult]("agent_chat_result_v1")
	schema.RegisterName[*cache.ExactCacheItem]("agent_exact_cache_item_v1")
	schema.RegisterName[*cache.SemanticCacheItem]("agent_semantic_cache_item_v1")
}

type StreamWriter = domain.StreamWriter
type Metrics = orchestratorobserve.Metrics
type State = domain.State
type Session = domain.Session

type Config struct {
	RateLimitPerMinute   int64
	ConversationWindow   int
	ExactCacheTTL        time.Duration
	SemanticCacheTTL     time.Duration
	SemanticCacheScore   float64
	SemanticCacheTopK    int
	RetrieveTopK         int
	RetrieveMinScore     float64
	ConfidenceThreshold  float64
	MaxAnswerTokens      int
	DefaultTenantID      string
	InterruptBeforeNodes []string
}

func DefaultConfig() Config {
	return Config{
		RateLimitPerMinute:  30,
		ConversationWindow:  5,
		ExactCacheTTL:       10 * time.Minute,
		SemanticCacheTTL:    30 * time.Minute,
		SemanticCacheScore:  0.9,
		SemanticCacheTopK:   20,
		RetrieveTopK:        8,
		RetrieveMinScore:    0.35,
		ConfidenceThreshold: 0.62,
		MaxAnswerTokens:     512,
		DefaultTenantID:     "default",
	}
}

type Dependencies struct {
	LLMs            LLMRouter
	Embedder        embedding.Embedder
	KnowledgeBase   rag.Searcher
	Skills          *agentskill.Registry
	Registry        *agenttool.Registry
	SessionService  *agentsession.Service
	ExactCache      cache.ExactCache
	SemanticCache   cache.SemanticCache
	RateLimiter     cache.RateLimiter
	CheckpointStore compose.CheckPointStore
	Logger          logger.LoggerV1
	Metrics         *Metrics
	Tracer          trace.Tracer
}

type Runtime struct {
	cfg             Config
	llms            LLMRouter
	embedder        embedding.Embedder
	knowledgeBase   rag.Searcher
	skills          *agentskill.Registry
	registry        *agenttool.Registry
	sessionService  *agentsession.Service
	exactCache      cache.ExactCache
	semanticCache   cache.SemanticCache
	rateLimiter     cache.RateLimiter
	checkpointStore compose.CheckPointStore
	logger          logger.LoggerV1
	metrics         *Metrics
	tracer          trace.Tracer
	callbackHandler callbacks.Handler
	runnable        compose.Runnable[struct{}, *State]
}

func NewMetrics(namespace string) *Metrics {
	return orchestratorobserve.NewMetrics(namespace)
}

type LLMRouter interface {
	Weak() model.ToolCallingChatModel
	Strong() model.ToolCallingChatModel
}
