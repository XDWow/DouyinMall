package graph

import (
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/trace"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/graph/observe"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/graph/prompt"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

func init() {
	schema.RegisterName[*FlowContext]("agent_flow_context_v2")
	schema.RegisterName[*ConversationState]("agent_conversation_state_v2")
	schema.RegisterName[*memory.Session]("agent_memory_session_v1")
	schema.RegisterName[*dto.ChatResponse]("agent_chat_response_v1")
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
type FlowContext = orchestratorstate.FlowContext
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
	SummaryTriggerTurns  int
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
		ConversationWindow:  8,
		L0CacheTTL:          10 * time.Minute,
		RetrieveTopK:        8,
		RetrieveMinScore:    0.35,
		RerankTopK:          4,
		ToolParallelism:     4,
		ConfidenceThreshold: 0.62,
		SummaryTriggerTurns: 12,
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

type Dependencies struct {
	Model           model.ToolCallingChatModel
	Embedder        embedding.Embedder
	Retriever       einoretriever.Retriever
	Registry        *agenttool.Registry
	SessionStore    memory.Store
	Summarizer      memory.Summarizer
	ExactCache      cache.ExactCache
	RateLimiter     cache.RateLimiter
	CheckpointStore cache.CheckpointStore
	Prompts         *PromptSet
	Logger          logger.LoggerV1
	Metrics         *Metrics
	Tracer          trace.Tracer
}

type Runtime struct {
	cfg             Config
	model           model.ToolCallingChatModel
	embedder        embedding.Embedder
	retriever       einoretriever.Retriever
	registry        *agenttool.Registry
	sessionStore    memory.Store
	summarizer      memory.Summarizer
	exactCache      cache.ExactCache
	rateLimiter     cache.RateLimiter
	checkpointStore cache.CheckpointStore
	prompts         *PromptSet
	logger          logger.LoggerV1
	metrics         *Metrics
	tracer          trace.Tracer
	callbackHandler callbacks.Handler
	runnable        compose.Runnable[map[string]any, *FlowContext]
}

func NewMetrics(namespace string) *Metrics {
	return orchestratorobserve.NewMetrics(namespace)
}

func NewDefaultPrompts() *PromptSet {
	return orchestratorprompt.NewDefault()
}
