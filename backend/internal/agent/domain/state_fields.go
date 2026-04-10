package domain

import "github.com/cloudwego/eino/schema"

type CacheState struct {
	AllowExact    bool   `json:"allow_exact"`
	AllowSemantic bool   `json:"allow_semantic"`
	IntentBucket  string `json:"intent_bucket,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

type IntentResult struct {
	Intent      Intent            `json:"intent"`
	Confidence  float64           `json:"confidence"`
	// Entities 本话解析出的键值，用来调工具；进 Session.Slots 仍只跟工具成功回写。
	Entities    map[string]string `json:"entities,omitempty"`
	NeedRewrite bool              `json:"need_rewrite"`
	NeedHandoff bool              `json:"need_handoff"`
	Reason      string            `json:"reason,omitempty"`
}

type RewriteResult struct {
	Query  string `json:"query"`
	Reason string `json:"reason,omitempty"`
}

type RetrievalResult struct {
	Documents []*schema.Document `json:"documents,omitempty"`
}

type ToolState struct {
	Plans        []ToolCallPlan    `json:"plans,omitempty"`
	CallMessage  *schema.Message   `json:"call_message,omitempty"`
	ToolMessages []*schema.Message `json:"tool_messages,omitempty"`
}

type AnswerResult struct {
	Reply         string  `json:"reply,omitempty"`
	Confidence    float64 `json:"confidence"`
	Source        string  `json:"source,omitempty"`
	CacheableHint *bool   `json:"cacheable_hint,omitempty"`
	Streamed      bool    `json:"streamed,omitempty"`
	NeedHandoff   bool    `json:"need_handoff"`
	HandoffReason string  `json:"handoff_reason,omitempty"`
	UsedToolNames []string `json:"-"`
}

type InterruptState struct {
	Payload map[string]any `json:"payload,omitempty"`
}
