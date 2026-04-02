package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
)

type Runner interface {
	Chat(ctx context.Context, req dto.ChatRequest) (*dto.ChatResponse, error)
	ChatStream(ctx context.Context, req dto.ChatRequest, writer graphstate.StreamWriter) (*dto.ChatResponse, error)
	CreateSession(ctx context.Context, userID int64, channel string) (*dto.Session, error)
	GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]dto.Message, int, error)
	ListSessions(ctx context.Context, userID int64, limit, offset int) ([]dto.Session, int, error)
	ClearSession(ctx context.Context, sessionID string) error
}

type Service interface {
	Chat(ctx context.Context, req dto.ChatRequest) (*dto.ChatResponse, error)
	ChatStream(ctx context.Context, req dto.ChatRequest, writer graphstate.StreamWriter) (*dto.ChatResponse, error)
	CreateSession(ctx context.Context, userID int64, channel string) (*dto.Session, error)
	GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]dto.Message, int, error)
	ListSessions(ctx context.Context, userID int64, limit, offset int) ([]dto.Session, int, error)
	ClearSession(ctx context.Context, sessionID string) error
}

type Facade struct {
	chat    *ChatUseCase
	session *SessionUseCase
}

func New(runner Runner) *Facade {
	return &Facade{
		chat:    NewChatUseCase(runner),
		session: NewSessionUseCase(runner),
	}
}

func (f *Facade) Chat(ctx context.Context, req dto.ChatRequest) (*dto.ChatResponse, error) {
	return f.chat.Execute(ctx, req)
}

func (f *Facade) ChatStream(ctx context.Context, req dto.ChatRequest, writer graphstate.StreamWriter) (*dto.ChatResponse, error) {
	return f.chat.Stream(ctx, req, writer)
}

func (f *Facade) CreateSession(ctx context.Context, userID int64, channel string) (*dto.Session, error) {
	return f.session.Create(ctx, userID, channel)
}

func (f *Facade) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]dto.Message, int, error) {
	return f.session.GetHistory(ctx, sessionID, limit, offset)
}

func (f *Facade) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]dto.Session, int, error) {
	return f.session.List(ctx, userID, limit, offset)
}

func (f *Facade) ClearSession(ctx context.Context, sessionID string) error {
	return f.session.Clear(ctx, sessionID)
}
