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
	knowledgebase "github.com/XDWow/DouyinMall/backend/internal/agent/infra/knowledgebase"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	agentmemory "github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

func init() {
	schema.RegisterName[*ConversationState]("agent_conversation_state_v2")
	schema.RegisterName[*domain.Session]("agent_domain_session_v1")
	schema.RegisterName[*domain.ChatResult]("agent_chat_result_v1")
	schema.RegisterName[*cache.ExactCacheItem]("agent_exact_cache_item_v1")
	schema.RegisterName[*cache.SemanticCacheItem]("agent_semantic_cache_item_v1")
}

type WorkflowRoute = orchestratorstate.WorkflowRoute

const (
	RouteUnknown             = orchestratorstate.RouteUnknown
	RouteOrderQuery          = orchestratorstate.RouteOrderQuery
	RouteReturnPolicy        = orchestratorstate.RouteReturnPolicy
	RouteInventory           = orchestratorstate.RouteInventory
	RouteProductInfo         = orchestratorstate.RouteProductInfo
	RouteReturnExchangeApply = orchestratorstate.RouteReturnExchangeApply
	RouteFallback            = orchestratorstate.RouteFallback
)

type FeatureFlags = orchestratorstate.FeatureFlags
type StreamWriter = orchestratorstate.StreamWriter
type PromptSet = orchestratorprompt.Set
type Metrics = orchestratorobserve.Metrics
type ConversationState = orchestratorstate.ConversationState
type SessionState = orchestratorstate.SessionState
type IntentResult = orchestratorstate.IntentResult
type RewriteResult = orchestratorstate.RewriteResult
type RetrievalResult = orchestratorstate.RetrievalResult
type ToolState = orchestratorstate.ToolState
type AnswerResult = orchestratorstate.AnswerResult

type Config struct {
	RateLimitPerMinute   int64
	ConversationWindow   int
	ExactCacheTTL        time.Duration
	SemanticCacheTTL     time.Duration
	SemanticCacheScore   float64
	SemanticCacheTopK    int
	RetrieveTopK         int
	RetrieveMinScore     float64
	RerankTopK           int
	ToolParallelism      int
	ConfidenceThreshold  float64
	MaxAnswerTokens      int
	StreamBuffer         int
	DefaultTenantID      string
	KBVersion            string
	FeatureFlags         FeatureFlags
	InterruptBeforeNodes []string
	InterruptAfterNodes  []string
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
		RerankTopK:          4,
		ToolParallelism:     4,
		ConfidenceThreshold: 0.62,
		MaxAnswerTokens:     512,
		StreamBuffer:        16,
		DefaultTenantID:     "default",
		KBVersion:           "default",
		FeatureFlags: FeatureFlags{
			OrderQuery:          true,
			ReturnPolicy:        true,
			Inventory:           true,
			ProductInfo:         true,
			ReturnExchangeApply: true,
		},
	}
}

// Dependencies 描述 Runtime 运行所需的外部依赖。
type Dependencies struct {
	Model           model.ToolCallingChatModel
	Embedder        embedding.Embedder
	KnowledgeBase   *knowledgebase.ManagedKnowledgeService
	Skills          *agentskill.Registry
	Registry        *agenttool.Registry
	Memory          *agentmemory.Manager
	ExactCache      cache.ExactCache
	SemanticCache   cache.SemanticCache
	RateLimiter     cache.RateLimiter
	CheckpointStore cache.CheckpointStore
	Prompts         *PromptSet
	Logger          logger.LoggerV1
	Metrics         *Metrics
	Tracer          trace.Tracer
}

// Runtime 是已编译主图的有状态封装。
type Runtime struct {
	cfg             Config
	model           model.ToolCallingChatModel
	embedder        embedding.Embedder
	knowledgeBase   *knowledgebase.ManagedKnowledgeService
	skills          *agentskill.Registry
	registry        *agenttool.Registry
	memory          *agentmemory.Manager
	exactCache      cache.ExactCache
	semanticCache   cache.SemanticCache
	rateLimiter     cache.RateLimiter
	checkpointStore cache.CheckpointStore
	prompts         *PromptSet
	logger          logger.LoggerV1
	metrics         *Metrics
	tracer          trace.Tracer
	callbackHandler callbacks.Handler
	runnable        compose.Runnable[map[string]any, *ConversationState]
}

func NewMetrics(namespace string) *Metrics {
	return orchestratorobserve.NewMetrics(namespace)
}

func NewDefaultPrompts() *PromptSet {
	return orchestratorprompt.NewDefault()
}
