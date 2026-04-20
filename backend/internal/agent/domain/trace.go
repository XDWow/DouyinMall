package domain

type Trace struct {
	TraceID              string
	CheckpointID         string
	CacheHit             bool
	RewrittenQuery       string
	Steps                []TraceStep
	SlowestStepNode      string
	SlowestStepLatencyMs int64
}

type TraceStep struct {
	Node      string
	Status    string
	LatencyMs int64
	Detail    string
}

type InterruptInfo struct {
	CheckpointID string
	InterruptID  string
	RerunNodes   []string
	Detail       map[string]any `json:"interrupt_info,omitempty"`
}
