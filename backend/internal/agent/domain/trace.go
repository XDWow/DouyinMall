package domain

type Trace struct {
	TraceID        string
	CheckpointID   string
	CacheHit       bool
	RewrittenQuery string
	Steps          []TraceStep
	// SlowestStep* 由编排层根据 Steps 汇总，用于单次请求瓶颈分析与指标归因（简历/看板可量化「最长耗时落在哪」）。
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
	RerunNodes   []string
}
