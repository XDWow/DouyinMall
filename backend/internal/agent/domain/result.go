package domain

type ChatResult struct {
	SessionID      string
	TraceID        string
	Status         ReplyStatus
	Reply          string
	Intent         Intent
	Confidence     float64
	NeedHandoff    bool
	HandoffReason  string
	References     []KnowledgeRef
	ToolExecutions []ToolExecution
	UsedToolNames  []string
	Trace          Trace
	Interrupt      *InterruptInfo
	// Interrupted 表示本轮以 compose 中断结束（与 Interrupt 字段一致时便于 JSON 直出）。
	Interrupted bool `json:"interrupted,omitempty"`
	// Streamed 表示至少有一部分回复通过流式通道下发（观测用）。
	Streamed bool `json:"streamed,omitempty"`
}
