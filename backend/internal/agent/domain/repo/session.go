package repo

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type SessionRepository interface {
	// Load 返回会话元信息和完整持久化历史。
	// 编排层再基于窗口大小决定传给模型多少条消息。
	Load(ctx context.Context, sessionID string) (*domain.Session, []domain.SessionMessage, error)
	Create(ctx context.Context, session domain.Session) error
	SaveRound(ctx context.Context, session domain.Session, userMessage domain.SessionMessage, assistantMessage domain.SessionMessage) error
	SaveMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage) error
	LoadAllMessages(ctx context.Context, sessionID string) ([]domain.SessionMessage, error)
	Clear(ctx context.Context, sessionID string) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error)
}
