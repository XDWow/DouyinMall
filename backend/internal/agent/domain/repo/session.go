package repo

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type SessionRepository interface {
	Load(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error)
	Create(ctx context.Context, session domain.Session) error
	SaveRound(ctx context.Context, session domain.Session, userMessage domain.Message, assistantMessage domain.Message) error
	SaveMessages(ctx context.Context, sessionID string, messages []domain.Message) error
	LoadAllMessages(ctx context.Context, sessionID string) ([]domain.Message, error)
	Clear(ctx context.Context, sessionID string) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error)
}
