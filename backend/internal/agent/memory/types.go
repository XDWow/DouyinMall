package memory

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
)

type Session struct {
	ID         string
	UserID     int64
	Channel    string
	Status     dto.SessionStatus
	Summary    string
	TotalTurns int
	Messages   []dto.Message
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (s *Session) RecentMessages(limit int) []dto.Message {
	if limit <= 0 || len(s.Messages) <= limit {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-limit:]
}

func (s *Session) LastMessagePreview() string {
	if len(s.Messages) == 0 {
		return ""
	}
	last := s.Messages[len(s.Messages)-1].Content
	runes := []rune(last)
	if len(runes) <= 64 {
		return last
	}
	return string(runes[:64]) + "..."
}

type Store interface {
	Load(ctx context.Context, sessionID string) (*Session, error)
	Create(ctx context.Context, session *Session) error
	Save(ctx context.Context, session *Session) error
	Clear(ctx context.Context, sessionID string) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]Session, int, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, session *Session) (string, error)
}

