package session

import (
	"context"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// Snapshot 表示一次会话加载得到的持久化快照。
// 它把“会话元信息”和“完整消息历史”明确放在一起，避免调用方自己拼语义。
type Snapshot struct {
	PersistedSession domain.Session
	Messages         []domain.SessionMessage
}

// SessionService 暴露编排层访问会话存储所需的最小能力。
type SessionService interface {
	LoadSnapshot(ctx context.Context, sessionID string) (*Snapshot, error)
	CreateSession(ctx context.Context, session domain.Session) error
	BuildRecentHistory(messages []domain.SessionMessage) []*schema.Message
}
