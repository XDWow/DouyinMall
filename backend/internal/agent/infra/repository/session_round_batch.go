package repository

import "github.com/XDWow/DouyinMall/backend/internal/agent/domain"

// SessionRoundSessionUpdate 批量落库时用于更新 agent_sessions 行的字段（非 MQ、非完整 domain.Session）。
type SessionRoundSessionUpdate struct {
	SessionID   string
	Status      string
	LastMessage string
	TotalTurns  int
	Slots       map[string]any
}

// SessionRoundPersistBatchItem 单轮写入：会话行更新 + 本轮消息（供 BatchPersistSessionRounds）。
type SessionRoundPersistBatchItem struct {
	Session  SessionRoundSessionUpdate
	Messages []domain.SessionMessage
}
