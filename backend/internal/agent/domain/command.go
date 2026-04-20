package domain

// TurnInput is an alias for the raw graph input type.
type TurnInput = *ChatInput

type WorkflowResumeInput struct {
	CheckpointID string
	InterruptID  string
	ResumeData   map[string]any
	UserID       int64
	SessionID    string
}

type CreateSessionCommand struct {
	UserID int64
}

type HistoryQuery struct {
	SessionID string
	Limit     int
	Offset    int
}

type SessionListQuery struct {
	UserID int64
	Limit  int
	Offset int
}

type ClearSessionCommand struct {
	SessionID string
}
