package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
)

type SessionLoadInput struct {
	SessionID string
	UserID    int64
}

type SessionLoadNode struct {
	service agentsession.SessionService
}

func NewSessionLoadNode(service agentsession.SessionService) *SessionLoadNode {
	return &SessionLoadNode{service: service}
}

func (n *SessionLoadNode) Invoke(ctx context.Context, in SessionLoadInput) (*domain.Session, error) {
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	if n == nil || n.service == nil {
		return defaultSession(sessionID, in.UserID), nil
	}

	snapshot, err := n.service.LoadSnapshot(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		sess := defaultSession(sessionID, in.UserID)
		meta := domain.SessionTableMeta{
			Status:     domain.SessionStatusActive,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			TotalTurns: 0,
		}
		if err := n.service.CreateSession(ctx, *sess, meta); err != nil {
			return nil, err
		}
		return sess, nil
	}
	if snapshot.Loaded.User.UserID != 0 && snapshot.Loaded.User.UserID != in.UserID {
		return nil, fmt.Errorf("session user mismatch")
	}

	sess := snapshot.Loaded.User
	sess.SessionID = sessionID
	if sess.UserID == 0 {
		sess.UserID = in.UserID
	}
	sess.Slots = domain.CloneAnyMap(snapshot.Loaded.Slots)
	if len(sess.RecentMessages) == 0 {
		sess.RecentMessages = messagesToTurns(snapshot.Messages, 10)
	}
	return &sess, nil
}

func defaultSession(sessionID string, userID int64) *domain.Session {
	return &domain.Session{
		SessionID: sessionID,
		UserID:    userID,
	}
}

func messagesToTurns(messages []domain.SessionMessage, maxTurns int) []domain.MessageTurn {
	if len(messages) == 0 {
		return nil
	}
	limit := len(messages)
	if maxTurns > 0 {
		if capSize := maxTurns * 2; capSize < limit {
			limit = capSize
		}
	}
	start := len(messages) - limit
	if start < 0 {
		start = 0
	}
	out := make([]domain.MessageTurn, 0, len(messages[start:]))
	for _, msg := range messages[start:] {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		out = append(out, domain.MessageTurn{
			Role:    msg.Role,
			Content: content,
		})
	}
	return out
}
