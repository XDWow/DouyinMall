package state

import (
	"context"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
)

type StreamWriter interface {
	Send(ctx context.Context, event dto.StreamEvent) error
}

type WorkflowRoute string

const (
	RouteUnknown             WorkflowRoute = "unknown"
	RouteOrderQuery          WorkflowRoute = "order_query"
	RouteReturnPolicy        WorkflowRoute = "return_policy"
	RouteInventory           WorkflowRoute = "inventory"
	RouteProductInfo         WorkflowRoute = "product_info"
	RouteReturnExchangeApply WorkflowRoute = "return_exchange_apply"
	RouteFallback            WorkflowRoute = "fallback"
)

type FeatureFlags struct {
	OrderQuery          bool
	ReturnPolicy        bool
	Inventory           bool
	ProductInfo         bool
	ReturnExchangeApply bool
}

type ConversationState struct {
	Flow *FlowContext
}

type SessionState struct {
	SessionID        string         `json:"session_id"`
	UserID           int64          `json:"user_id"`
	TenantID         string         `json:"tenant_id"`
	RawQuery         string         `json:"raw_query"`
	RewrittenQuery   string         `json:"rewritten_query,omitempty"`
	Intent           dto.Intent     `json:"intent"`
	IntentConfidence float64        `json:"intent_confidence"`
	Route            WorkflowRoute  `json:"route"`
	Slots            map[string]any `json:"slots,omitempty"`
	MissingSlots     []string       `json:"missing_slots,omitempty"`
	AwaitingUser     bool           `json:"awaiting_user"`
	AwaitingConfirm  bool           `json:"awaiting_confirm"`
	FinalAnswer      string         `json:"final_answer,omitempty"`
	ErrorCode        string         `json:"error_code,omitempty"`
	CacheHitLevel    string         `json:"cache_hit_level,omitempty"`
	KBVersion        string         `json:"kb_version,omitempty"`
	FeatureFlags     FeatureFlags   `json:"feature_flags"`
	ReadOnly         bool           `json:"read_only"`
	ResumeFromCP     bool           `json:"resume_from_checkpoint"`
	NeedHandoff      bool           `json:"need_handoff"`
	HandoffReason    string         `json:"handoff_reason,omitempty"`
}

type FlowContext struct {
	StartedAt  time.Time         `json:"-"`
	TraceID    string            `json:"trace_id"`
	Request    dto.ChatRequest   `json:"request"`
	Session    *memory.Session   `json:"session,omitempty"`
	Response   *dto.ChatResponse `json:"response,omitempty"`
	Checkpoint string            `json:"checkpoint,omitempty"`

	StreamWriter StreamWriter                     `json:"-"`
	Recorder     *agenttool.SafeExecutionRecorder `json:"-"`

	State     SessionState    `json:"state"`
	Intent    IntentDecision  `json:"intent"`
	Rewrite   RewriteDecision `json:"rewrite"`
	Retrieval RetrievalResult `json:"retrieval"`
	Tool      ToolDecision    `json:"tool"`
	Answer    AnswerResult    `json:"answer"`
}

type IntentDecision struct {
	Intent      dto.Intent        `json:"intent"`
	Confidence  float64           `json:"confidence"`
	Entities    map[string]string `json:"entities,omitempty"`
	NeedRewrite bool              `json:"need_rewrite"`
	Reason      string            `json:"reason,omitempty"`
}

type RewriteDecision struct {
	Query  string `json:"query"`
	Reason string `json:"reason,omitempty"`
}

type RetrievalResult struct {
	Query      string             `json:"query,omitempty"`
	Documents  []*schema.Document `json:"documents,omitempty"`
	References []dto.KnowledgeRef `json:"references,omitempty"`
}

type ToolDecision struct {
	Plans           []dto.ToolCallPlan `json:"plans,omitempty"`
	DecisionMessage *schema.Message    `json:"decision_message,omitempty"`
	ToolMessages    []*schema.Message  `json:"tool_messages,omitempty"`
}

type AnswerResult struct {
	Reply      string  `json:"reply,omitempty"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source,omitempty"`
}

type InitOptions struct {
	KBVersion    string
	FeatureFlags FeatureFlags
}

func ProcessConversationState(ctx context.Context, handler func(*ConversationState) error) error {
	return compose.ProcessState[*ConversationState](ctx, func(_ context.Context, state *ConversationState) error {
		if state == nil {
			return nil
		}
		return handler(state)
	})
}
