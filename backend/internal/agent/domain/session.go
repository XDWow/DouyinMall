package domain

import "time"

type Message struct {
	ID         string
	SessionID  string
	Role       Role
	Content    string
	Intent     Intent
	Confidence float64
	CreatedAt  time.Time
}

type Session struct {
	SessionID   string
	UserID      int64
	Status      SessionStatus
	LastMessage string
	TotalTurns  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type KnowledgeRef struct {
	ID       string
	Title    string
	Snippet  string
	Category string
	Score    float64
	Metadata map[string]string
}

type ToolCallPlan struct {
	Name      string
	Arguments map[string]any
	Reason    string
	RawJSON   string
}

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

type Trace struct {
	TraceID        string
	CheckpointID   string
	CacheHit       bool
	RewrittenQuery string
	Steps          []TraceStep
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
