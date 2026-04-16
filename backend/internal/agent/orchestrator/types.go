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
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

func init() {
	schema.RegisterName[*domain.State]("agent_state_v2")
	schema.RegisterName[*domain.Session]("agent_domain_session_v1")
	schema.RegisterName[*domain.ChatResult]("agent_chat_result_v1")
	schema.RegisterName[*cache.ExactCacheItem]("agent_exact_cache_item_v1")
	schema.RegisterName[*cache.SemanticCacheItem]("agent_semantic_cache_item_v1")
}

type WorkflowRoute = domain.WorkflowRoute

const (
	RouteUnknown             = domain.RouteUnknown
	RouteOrderQuery          = domain.RouteOrderQuery
	RouteReturnPolicy        = domain.RouteReturnPolicy
	RouteInventory           = domain.RouteInventory
	RouteProductInfo         = domain.RouteProductInfo
	RouteAddToCart           = domain.RouteAddToCart
	RouteReturnExchangeApply = domain.RouteReturnExchangeApply
	RouteBaseQA              = domain.RouteBaseQA
)

type StreamWriter = domain.StreamWriter
type PromptSet = orchestratorprompt.Set
type Metrics = orchestratorobserve.Metrics
type State = domain.State
type Session = domain.Session
type IntentResult = domain.IntentResult
type RewriteResult = domain.RewriteResult
type RetrievalResult = domain.RetrievalResult
type ToolState = domain.ToolState
type AnswerResult = domain.AnswerResult

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
		RateLimitPerMinute: 30,
		ConversationWindow: 5,
		// v1 仅缺参追问：在「会走 MissingSlots / applySubgraphSlotWait」的子图入口打断并 checkpoint，补参后同 checkpoint_id + interrupt_id 恢复。
		InterruptBeforeNodes: []string{
			"InventoryGraph",
			"ProductInfoGraph",
			"AddToCartGraph",
			"ReturnExchangeGraph",
		},
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
	Model           model.ToolCallingChatModel
	Embedder        embedding.Embedder
	KnowledgeBase   *rag.ManagedKnowledgeService
	Skills          *agentskill.Registry
	Registry        *agenttool.Registry
	SessionService  *agentsession.Service
	ExactCache      cache.ExactCache
	SemanticCache   cache.SemanticCache
	RateLimiter     cache.RateLimiter
	CheckpointStore compose.CheckPointStore
	Prompts         *PromptSet
	Logger          logger.LoggerV1
	Metrics         *Metrics
	Tracer          trace.Tracer
}

type Runtime struct {
	cfg             Config
	model           model.ToolCallingChatModel
	embedder        embedding.Embedder
	knowledgeBase   *rag.ManagedKnowledgeService
	skills          *agentskill.Registry
	registry        *agenttool.Registry
	sessionService  *agentsession.Service
	exactCache      cache.ExactCache
	semanticCache   cache.SemanticCache
	rateLimiter     cache.RateLimiter
	checkpointStore compose.CheckPointStore
	prompts         *PromptSet
	logger          logger.LoggerV1
	metrics         *Metrics
	tracer          trace.Tracer
	callbackHandler callbacks.Handler
	runnable        compose.Runnable[map[string]any, *State]
}

func NewMetrics(namespace string) *Metrics {
	return orchestratorobserve.NewMetrics(namespace)
}

func NewDefaultPrompts() *PromptSet {
	return orchestratorprompt.NewDefault()
}
