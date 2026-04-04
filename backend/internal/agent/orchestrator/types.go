package orchestrator

import (
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/trace"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentmemory "github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

func init() {
	schema.RegisterName[*ConversationState]("agent_conversation_state_v2")
	schema.RegisterName[*domain.Session]("agent_domain_session_v1")
	schema.RegisterName[*domain.ChatResult]("agent_chat_result_v1")
	schema.RegisterName[*cache.ExactCacheItem]("agent_exact_cache_item_v1")
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
type IntentDecision = orchestratorstate.IntentDecision
type RewriteDecision = orchestratorstate.RewriteDecision
type RetrievalResult = orchestratorstate.RetrievalResult
type ToolDecision = orchestratorstate.ToolDecision
type AnswerResult = orchestratorstate.AnswerResult

type Config struct {
	RateLimitPerMinute   int64
	ConversationWindow   int
	L0CacheTTL           time.Duration
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
		L0CacheTTL:          10 * time.Minute,
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

// Dependencies holds every external component the orchestrator Runtime needs.
// Construct once at application startup and inject into [NewRuntime].
type Dependencies struct {
	Model           model.ToolCallingChatModel
	Embedder        embedding.Embedder
	Retriever       einoretriever.Retriever
	Registry        *agenttool.Registry
	Memory          *agentmemory.Manager
	ExactCache      cache.ExactCache
	RateLimiter     cache.RateLimiter
	CheckpointStore cache.CheckpointStore
	Prompts         *PromptSet
	Logger          logger.LoggerV1
	Metrics         *Metrics
	Tracer          trace.Tracer
}

// Runtime is the stateful orchestrator that wraps the compiled eino graph.
// Create via [NewRuntime]; the zero value is not usable.
type Runtime struct {
	cfg             Config
	model           model.ToolCallingChatModel
	embedder        embedding.Embedder
	retriever       einoretriever.Retriever
	registry        *agenttool.Registry
	memory          *agentmemory.Manager
	exactCache      cache.ExactCache
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
