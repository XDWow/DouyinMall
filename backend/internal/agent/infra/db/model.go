package db

import "time"

// 以下为 agent MySQL 表的 GORM 模型，与 domain 层分离；仓储通过映射与 domain 互转。

type Session struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID   string    `gorm:"uniqueIndex;type:varchar(64);not null"`
	UserID      int64     `gorm:"index;not null"`
	Status      string    `gorm:"type:varchar(16);not null"`
	LastMessage string    `gorm:"type:varchar(255);not null;default:''"`
	TotalTurns  int       `gorm:"not null;default:0"`
	SlotsJSON   string    `gorm:"type:longtext"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (Session) TableName() string { return "agent_sessions" }

type Message struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID  string    `gorm:"index:idx_session_created;type:varchar(64);not null"`
	Role       string    `gorm:"type:varchar(16);not null"`
	Content    string    `gorm:"type:text;not null"`
	Intent     string    `gorm:"type:varchar(32);not null"`
	Confidence float64   `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"index:idx_session_created;autoCreateTime"`
}

func (Message) TableName() string { return "agent_messages" }

type KnowledgeChunk struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)"`
	KnowledgeID string    `gorm:"index;type:varchar(64);not null"`
	Title       string    `gorm:"type:varchar(255);not null"`
	Category    string    `gorm:"index;type:varchar(64);not null"`
	Content     string    `gorm:"type:longtext;not null"`
	Snippet     string    `gorm:"type:text"`
	Embedding   string    `gorm:"type:longtext;not null"`
	Metadata    string    `gorm:"type:longtext"`
	Enabled     bool      `gorm:"not null;default:true"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (KnowledgeChunk) TableName() string { return "agent_knowledge_chunks" }
