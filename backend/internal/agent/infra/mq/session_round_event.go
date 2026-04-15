package mq

import (
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentrepository "github.com/XDWow/DouyinMall/backend/internal/agent/infra/repository"
)

// SessionRoundSessionPayload Kafka 载荷中与 agent_sessions 行对应的会话片段（MQ 自有契约，非 domain.Session）。
type SessionRoundSessionPayload struct {
	SessionID   string         `json:"session_id"`
	UserID      int64          `json:"user_id"`
	Status      string         `json:"status"`
	LastMessage string         `json:"last_message"`
	TotalTurns  int            `json:"total_turns"`
	Slots       map[string]any `json:"slots,omitempty"`
}

// SessionRoundMessagePayload Kafka 载荷中的单条消息（MQ 自有契约，非 domain.SessionMessage）。
type SessionRoundMessagePayload struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	Intent     string    `json:"intent"`
	Confidence float64   `json:"confidence"`
	CreatedAt  time.Time `json:"created_at"`
}

// SessionRoundPersistEvent 单轮会话异步落库的 Kafka 消息体。
type SessionRoundPersistEvent struct {
	Session  SessionRoundSessionPayload   `json:"session"`
	Messages []SessionRoundMessagePayload `json:"messages"`
}

// NewSessionRoundPersistEvent 从领域对象组装 MQ 事件；空内容消息不会加入列表。
func NewSessionRoundPersistEvent(session domain.Session, userMessage, assistantMessage domain.SessionMessage) SessionRoundPersistEvent {
	ev := SessionRoundPersistEvent{
		Session: SessionRoundSessionPayload{
			SessionID:   session.SessionID,
			UserID:      session.UserID,
			Status:      string(session.Status),
			LastMessage: session.LastMessage,
			TotalTurns:  session.TotalTurns,
			Slots:       session.Slots,
		},
	}
	if strings.TrimSpace(userMessage.Content) != "" {
		ev.Messages = append(ev.Messages, sessionMessageToPayload(userMessage))
	}
	if strings.TrimSpace(assistantMessage.Content) != "" {
		ev.Messages = append(ev.Messages, sessionMessageToPayload(assistantMessage))
	}
	return ev
}

func sessionMessageToPayload(m domain.SessionMessage) SessionRoundMessagePayload {
	return SessionRoundMessagePayload{
		ID:         m.ID,
		SessionID:  m.SessionID,
		Role:       string(m.Role),
		Content:    m.Content,
		Intent:     string(m.Intent),
		Confidence: m.Confidence,
		CreatedAt:  m.CreatedAt,
	}
}

// SessionRoundEventsToRepoBatch 将 MQ 反序列化结果转为仓储批量落库入参（边界：mq 契约 → domain + repo DTO）。
func SessionRoundEventsToRepoBatch(events []SessionRoundPersistEvent) []agentrepository.SessionRoundPersistBatchItem {
	out := make([]agentrepository.SessionRoundPersistBatchItem, 0, len(events))
	for _, e := range events {
		item := agentrepository.SessionRoundPersistBatchItem{
			Session: agentrepository.SessionRoundSessionUpdate{
				SessionID:   e.Session.SessionID,
				Status:      e.Session.Status,
				LastMessage: e.Session.LastMessage,
				TotalTurns:  e.Session.TotalTurns,
				Slots:       e.Session.Slots,
			},
		}
		for _, m := range e.Messages {
			item.Messages = append(item.Messages, domain.SessionMessage{
				ID:         m.ID,
				SessionID:  m.SessionID,
				Role:       domain.Role(m.Role),
				Content:    m.Content,
				Intent:     domain.Intent(m.Intent),
				Confidence: m.Confidence,
				CreatedAt:  m.CreatedAt,
			})
		}
		out = append(out, item)
	}
	return out
}
