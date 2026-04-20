package session

import (
	"context"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// Snapshot 表示一次会话加载得到的持久化快照。
type Snapshot struct {
	Loaded   domain.LoadedSession
	Messages []domain.SessionMessage
}

// SessionService 暴露编排层访问会话存储所需的最小能力。
type SessionService interface {
	LoadSnapshot(ctx context.Context, sessionID string) (*Snapshot, error)
	CreateSession(ctx context.Context, user domain.Session, meta domain.SessionTableMeta) error
	BuildRecentHistory(messages []domain.SessionMessage) []*schema.Message
}
