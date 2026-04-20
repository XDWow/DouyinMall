package domain

import "time"

// SessionTableMeta 对应 agent_sessions 表级字段（列表/统计），不是对话语义本身。
type SessionTableMeta struct {
	Status      SessionStatus
	LastMessage string
	TotalTurns  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SessionListItem 列表查询一行：用户会话上下文 + 表元信息。
type SessionListItem struct {
	Context Session
	Meta    SessionTableMeta
}

// LoadedSession 仓储一次加载：用户会话 + 表元 + 槽位 blob（工具态与内嵌 _usr_ctx 已剥离）。
type LoadedSession struct {
	User  Session
	Meta  SessionTableMeta
	Slots map[string]any
}

// RoundPersistInput 落库一轮时提交的会话片段（用户列 + slots_json）。
type RoundPersistInput struct {
	User  Session
	Meta  SessionTableMeta
	Slots map[string]any
}
