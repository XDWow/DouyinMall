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
	Trace          Trace
	Interrupt      *InterruptInfo
}
