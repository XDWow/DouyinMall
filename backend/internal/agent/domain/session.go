package domain

import (
	"time"

	"github.com/cloudwego/eino/schema"
)

type SessionMessage struct {
	ID         string
	SessionID  string
	Role       Role
	Content    string
	Intent     Intent
	Confidence float64
	CreatedAt  time.Time
}

type Message = SessionMessage

type PendingSelection struct {
	Kind    string            `json:"kind,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

type CurrentRefs struct {
	ProductID string `json:"product_id,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
}

// Session 会话：持久化字段 + 本轮编排字段。
type Session struct {
	SessionID   string
	UserID      int64
	Status      SessionStatus
	LastMessage string
	TotalTurns  int
	// Slots 工作集：上轮持久化 + 本回合工具结果回写；业务 ID 以工具/CurrentRefs 为准。意图抽槽只在子图副本里拼参；与 CurrentRefs 对齐在 Intent 节点阶段完成，不预灌未确认解析。
	Slots map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time

	TenantID          string
	RawQuery          string
	Messages          []*schema.Message           `json:"messages,omitempty"`
	PendingSelections map[string]PendingSelection `json:"pending_selections,omitempty"`
	CurrentRefs       CurrentRefs                 `json:"current_refs"`
	Intent            Intent                      `json:"intent"`
	IntentConfidence  float64                     `json:"intent_confidence"`
	Route             WorkflowRoute               `json:"route"`
	MissingSlots      []string                    `json:"missing_slots,omitempty"`
	AwaitingUser      bool                        `json:"awaiting_user"`
	AwaitingConfirm   bool                        `json:"awaiting_confirm"`
	FinalAnswer       string                      `json:"final_answer,omitempty"`
	ErrorCode         string                      `json:"error_code,omitempty"`
	CacheHitLevel     string                      `json:"cache_hit_level,omitempty"`
	ReadOnly          bool                        `json:"read_only"`
	ResumeFromCP      bool                        `json:"resume_from_checkpoint"`
	NeedHandoff       bool                        `json:"need_handoff"`
	HandoffReason     string                      `json:"handoff_reason,omitempty"`
}

func (s *Session) ApplyPersistedFields(src Session) {
	if s == nil {
		return
	}
	s.SessionID = src.SessionID
	s.UserID = src.UserID
	s.Status = src.Status
	s.LastMessage = src.LastMessage
	s.TotalTurns = src.TotalTurns
	s.Slots = src.Slots
	s.CreatedAt = src.CreatedAt
	s.UpdatedAt = src.UpdatedAt
}
