package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

type runtime interface {
	Chat(ctx context.Context, command domain.ChatCommand) (*domain.ChatResult, error)
	ChatStream(ctx context.Context, command domain.ChatCommand, writer graphstate.StreamWriter) (*domain.ChatResult, error)
	CreateSession(ctx context.Context, userID int64) (*domain.Session, error)
	GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]domain.Message, int, error)
	ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error)
	ClearSession(ctx context.Context, sessionID string) error
}

type Service interface {
	Chat(ctx context.Context, input ChatInput) (*ChatOutput, error)
	ChatStream(ctx context.Context, input ChatInput, writer graphstate.StreamWriter) (*ChatOutput, error)
	CreateSession(ctx context.Context, input CreateSessionInput) (*SessionOutput, error)
	GetHistory(ctx context.Context, input HistoryInput) (*HistoryOutput, error)
	ListSessions(ctx context.Context, input ListSessionsInput) (*SessionListOutput, error)
	ClearSession(ctx context.Context, input ClearSessionInput) error
}

type Facade struct {
	chat    *ChatUseCase
	session *SessionUseCase
}

func New(runtime runtime) *Facade {
	return &Facade{
		chat:    NewChatUseCase(runtime),
		session: NewSessionUseCase(runtime),
	}
}

func (f *Facade) Chat(ctx context.Context, input ChatInput) (*ChatOutput, error) {
	return f.chat.Execute(ctx, input)
}

func (f *Facade) ChatStream(ctx context.Context, input ChatInput, writer graphstate.StreamWriter) (*ChatOutput, error) {
	return f.chat.Stream(ctx, input, writer)
}

func (f *Facade) CreateSession(ctx context.Context, input CreateSessionInput) (*SessionOutput, error) {
	return f.session.Create(ctx, input)
}

func (f *Facade) GetHistory(ctx context.Context, input HistoryInput) (*HistoryOutput, error) {
	return f.session.GetHistory(ctx, input)
}

func (f *Facade) ListSessions(ctx context.Context, input ListSessionsInput) (*SessionListOutput, error) {
	return f.session.List(ctx, input)
}

func (f *Facade) ClearSession(ctx context.Context, input ClearSessionInput) error {
	return f.session.Clear(ctx, input)
}

type ChatInput struct {
	SessionID   string
	UserID      int64
	Message     string
	ResumeToken string
	Metadata    map[string]string
}

type ChatOutput struct {
	SessionID      string
	TraceID        string
	Status         domain.ReplyStatus
	Reply          string
	Intent         domain.Intent
	Confidence     float64
	NeedHandoff    bool
	HandoffReason  string
	References     []KnowledgeRef
	ToolExecutions []ToolExecution
	Trace          Trace
	Interrupt      *InterruptInfo
}

type CreateSessionInput struct {
	UserID int64
}

type SessionOutput struct {
	SessionID   string
	UserID      int64
	Status      domain.SessionStatus
	LastMessage string
	TotalTurns  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type HistoryInput struct {
	SessionID string
	Limit     int
	Offset    int
}

type HistoryOutput struct {
	Messages []MessageOutput
	Total    int
}

type ListSessionsInput struct {
	UserID int64
	Limit  int
	Offset int
}

type SessionListOutput struct {
	Sessions []SessionOutput
	Total    int
}

type ClearSessionInput struct {
	SessionID string
}

type MessageOutput struct {
	ID         string
	SessionID  string
	Role       domain.Role
	Content    string
	Intent     domain.Intent
	Confidence float64
	CreatedAt  time.Time
}

type KnowledgeRef struct {
	ID       string
	Title    string
	Snippet  string
	Category string
	Score    float64
	Metadata map[string]string
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

type StreamEvent = graphstate.StreamEvent

