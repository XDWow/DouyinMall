package state

import (
	"context"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
)

type StreamWriter interface {
	Send(ctx context.Context, event StreamEvent) error
}

type StreamEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

type WorkflowRoute string

const (
	RouteUnknown             WorkflowRoute = "unknown"
	RouteOrderQuery          WorkflowRoute = "order_query"
	RouteInventory           WorkflowRoute = "inventory"
	RouteProductInfo         WorkflowRoute = "product_info"
	RouteAddToCart           WorkflowRoute = "add_to_cart"
	RouteReturnPolicy        WorkflowRoute = "return_policy"
	RouteReturnExchangeApply WorkflowRoute = "return_exchange_apply"
	RouteBaseQA              WorkflowRoute = "base_qa"
)

type FeatureFlags struct {
	OrderQuery          bool
	Inventory           bool
	ProductInfo         bool
	AddToCart           bool
	ReturnPolicy        bool
	ReturnExchangeApply bool
}

type Session struct {
	SessionID         string                      `json:"session_id"`
	UserID            int64                       `json:"user_id"`
	TenantID          string                      `json:"tenant_id"`
	RawQuery          string                      `json:"raw_query"`
	Messages          []*schema.Message           `json:"messages,omitempty"`
	PendingSelections map[string]PendingSelection `json:"pending_selections,omitempty"`
	CurrentRefs       CurrentRefs                 `json:"current_refs"`
	Intent            domain.Intent               `json:"intent"`
	IntentConfidence  float64                     `json:"intent_confidence"`
	Route             WorkflowRoute               `json:"route"`
	Slots             map[string]any              `json:"slots,omitempty"`
	MissingSlots      []string                    `json:"missing_slots,omitempty"`
	AwaitingUser      bool                        `json:"awaiting_user"`
	AwaitingConfirm   bool                        `json:"awaiting_confirm"`
	FinalAnswer       string                      `json:"final_answer,omitempty"`
	ErrorCode         string                      `json:"error_code,omitempty"`
	CacheHitLevel     string                      `json:"cache_hit_level,omitempty"`
	KBVersion         string                      `json:"kb_version,omitempty"`
	FeatureFlags      FeatureFlags                `json:"feature_flags"`
	ReadOnly          bool                        `json:"read_only"`
	ResumeFromCP      bool                        `json:"resume_from_checkpoint"`
	NeedHandoff       bool                        `json:"need_handoff"`
	HandoffReason     string                      `json:"handoff_reason,omitempty"`
}

type State struct {
	StartedAt   time.Time          `json:"-"`
	TraceID     string             `json:"trace_id"`
	Request     domain.ChatCommand `json:"request"`
	SessionMeta *domain.Session    `json:"session_meta,omitempty"`
	Response    *domain.ChatResult `json:"response,omitempty"`
	Checkpoint  string             `json:"checkpoint,omitempty"`
	Interrupt   *InterruptState    `json:"interrupt,omitempty"`

	StreamWriter StreamWriter                     `json:"-"`
	Recorder     *agenttool.SafeExecutionRecorder `json:"-"`

	Session   Session         `json:"session"`
	Cache     CacheState      `json:"cache"`
	Intent    IntentResult    `json:"intent"`
	Skill     SkillState      `json:"skill"`
	Rewrite   RewriteResult   `json:"rewrite"`
	Retrieval RetrievalResult `json:"retrieval"`
	Tool      ToolState       `json:"tool"`
	Answer    AnswerResult    `json:"answer"`
}

type CacheState struct {
	AllowExact    bool   `json:"allow_exact"`
	AllowSemantic bool   `json:"allow_semantic"`
	IntentBucket  string `json:"intent_bucket,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

type IntentResult struct {
	Intent      domain.Intent     `json:"intent"`
	Confidence  float64           `json:"confidence"`
	Entities    map[string]string `json:"entities,omitempty"`
	NeedRewrite bool              `json:"need_rewrite"`
	Reason      string            `json:"reason,omitempty"`
}

type RewriteResult struct {
	Query  string `json:"query"`
	Reason string `json:"reason,omitempty"`
}

type PendingSelection struct {
	Kind    string            `json:"kind,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

type CurrentRefs struct {
	ProductID string `json:"product_id,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
}

type SkillState struct {
	Names []string `json:"names,omitempty"`
}

type RetrievalResult struct {
	Documents []*schema.Document `json:"documents,omitempty"`
}

type ToolState struct {
	Plans        []domain.ToolCallPlan `json:"plans,omitempty"`
	CallMessage  *schema.Message       `json:"call_message,omitempty"`
	ToolMessages []*schema.Message     `json:"tool_messages,omitempty"`
}

type AnswerResult struct {
	Reply         string  `json:"reply,omitempty"`
	Confidence    float64 `json:"confidence"`
	Source        string  `json:"source,omitempty"`
	CacheableHint *bool   `json:"cacheable_hint,omitempty"`
	Streamed      bool    `json:"streamed,omitempty"`
}

type InterruptState struct {
	Payload map[string]any `json:"payload,omitempty"`
}

type InitOptions struct {
	KBVersion    string
	FeatureFlags FeatureFlags
}

func ProcessState(ctx context.Context, handler func(*State) error) error {
	return compose.ProcessState(ctx, func(_ context.Context, state *State) error {
		if state == nil {
			return nil
		}
		return handler(state)
	})
}
