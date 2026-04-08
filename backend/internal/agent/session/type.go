package session

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/cloudwego/eino/schema"
)

type SessionService interface {
	LoadSession(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error)
	CreateSession(ctx context.Context, session domain.Session) error
	RecentSchemaMessages(messages []domain.Message) []*schema.Message
}
