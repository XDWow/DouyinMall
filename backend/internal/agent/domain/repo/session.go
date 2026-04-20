package repo

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type SessionRepository interface {
	Load(ctx context.Context, sessionID string) (*domain.LoadedSession, []domain.SessionMessage, error)
	Create(ctx context.Context, user domain.Session, meta domain.SessionTableMeta, slots map[string]any) error
	SaveRound(ctx context.Context, in domain.RoundPersistInput, userMessage domain.SessionMessage, assistantMessage domain.SessionMessage) error
	SaveMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage) error
	LoadAllMessages(ctx context.Context, sessionID string) ([]domain.SessionMessage, error)
	Clear(ctx context.Context, sessionID string) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.SessionListItem, int, error)
}
