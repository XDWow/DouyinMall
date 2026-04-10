package domain

import "context"

type StreamWriter interface {
	Send(ctx context.Context, event StreamEvent) error
}

type StreamEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

type ToolExecutionSink interface {
	Record(exec ToolExecution)
	Snapshot() []ToolExecution
}
