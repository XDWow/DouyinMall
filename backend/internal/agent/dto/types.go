package dto

import "time"

type Intent string

const (
	IntentUnknown         Intent = "unknown"
	IntentFAQ             Intent = "faq"
	IntentProductSearch   Intent = "product_search"
	IntentOrderQuery      Intent = "order_query"
	IntentAddToCart       Intent = "add_to_cart"
	IntentPolicy          Intent = "policy"
	IntentComplaint       Intent = "complaint"
	IntentHandoff         Intent = "handoff"
	IntentChitchat        Intent = "chitchat"
	IntentUnsupported     Intent = "unsupported"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type ReplyStatus string

const (
	ReplyStatusAnswered ReplyStatus = "answered"
	ReplyStatusFallback ReplyStatus = "fallback"
	ReplyStatusHandoff  ReplyStatus = "handoff"
)

type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
	SessionStatusClosed SessionStatus = "closed"
	SessionStatusHuman  SessionStatus = "human"
)

type ChatRequest struct {
	SessionID   string            `json:"session_id"`
	UserID      int64             `json:"user_id"`
	Message     string            `json:"message"`
	Channel     string            `json:"channel"`
	ResumeToken string            `json:"resume_token,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ChatResponse struct {
	SessionID      string          `json:"session_id"`
	TraceID        string          `json:"trace_id"`
	Status         ReplyStatus     `json:"status"`
	Reply          string          `json:"reply"`
	Intent         Intent          `json:"intent"`
	Confidence     float64         `json:"confidence"`
	NeedHandoff    bool            `json:"need_handoff"`
	HandoffReason  string          `json:"handoff_reason,omitempty"`
	References     []KnowledgeRef  `json:"references,omitempty"`
	ToolExecutions []ToolExecution `json:"tool_executions,omitempty"`
	Trace          Trace           `json:"trace"`
	Interrupt      *InterruptInfo  `json:"interrupt,omitempty"`
}

type Message struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Role       Role      `json:"role"`
	Content    string    `json:"content"`
	Intent     Intent    `json:"intent,omitempty"`
	Confidence float64   `json:"confidence,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Session struct {
	SessionID   string        `json:"session_id"`
	UserID      int64         `json:"user_id"`
	Channel     string        `json:"channel"`
	Status      SessionStatus `json:"status"`
	Summary     string        `json:"summary,omitempty"`
	LastMessage string        `json:"last_message,omitempty"`
	TotalTurns  int           `json:"total_turns"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type KnowledgeRef struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Snippet   string            `json:"snippet"`
	Category  string            `json:"category"`
	Score     float64           `json:"score"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type ToolCallPlan struct {
	Name      string            `json:"name"`
	Arguments map[string]any    `json:"arguments,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	RawJSON   string            `json:"raw_json,omitempty"`
}

type ToolExecution struct {
	Name       string            `json:"name"`
	Arguments  map[string]any    `json:"arguments,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Success    bool              `json:"success"`
	Result     string            `json:"result,omitempty"`
	Error      string            `json:"error,omitempty"`
	LatencyMs  int64             `json:"latency_ms"`
	OccurredAt time.Time         `json:"occurred_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Trace struct {
	TraceID        string      `json:"trace_id"`
	CheckpointID   string      `json:"checkpoint_id,omitempty"`
	CacheHit       bool        `json:"cache_hit"`
	RewrittenQuery string      `json:"rewritten_query,omitempty"`
	Steps          []TraceStep `json:"steps,omitempty"`
}

type TraceStep struct {
	Node      string `json:"node"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Detail    string `json:"detail,omitempty"`
}

type InterruptInfo struct {
	CheckpointID string   `json:"checkpoint_id"`
	RerunNodes   []string `json:"rerun_nodes,omitempty"`
}

type StreamEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

