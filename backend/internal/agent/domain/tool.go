package domain

import "time"

type ToolExecution struct {
	Name       string
	Arguments  map[string]any
	Reason     string
	Success    bool
	Result     string
	Error      string
	LatencyMs  int64
	OccurredAt time.Time
	Metadata   map[string]string
}
