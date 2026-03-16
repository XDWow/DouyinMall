package db

import "time"

type Session struct {
	ID                 uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID          string    `gorm:"uniqueIndex;type:varchar(64);not null"`
	UserID             uint64    `gorm:"index;not null"`
	Channel            string    `gorm:"type:varchar(32);not null;default:web"`
	Status             uint8     `gorm:"not null;default:1"`
	LowConfidenceTurns int       `gorm:"not null;default:0"`
	CreatedAt          time.Time `gorm:"autoCreateTime"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime"`
}

func (Session) TableName() string { return "agent_sessions" }

type Message struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID  string    `gorm:"index:idx_session_created;type:varchar(64);not null"`
	Role       string    `gorm:"type:varchar(16);not null"`
	Content    string    `gorm:"type:text;not null"`
	Intent     int8      `gorm:"not null;default:0"`
	Confidence float32   `gorm:"not null;default:0"`
	TokensUsed int       `gorm:"not null;default:0"`
	LatencyMs  int       `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"index:idx_session_created;autoCreateTime"`
}

func (Message) TableName() string { return "agent_messages" }

type KnowledgeItem struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	Title     string    `gorm:"type:varchar(256);not null"`
	Content   string    `gorm:"type:text;not null"`
	Category  string    `gorm:"index;type:varchar(64);not null"`
	VectorID  string    `gorm:"type:varchar(64);default:''"`
	Status    int8      `gorm:"index;not null;default:1"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (KnowledgeItem) TableName() string { return "knowledge_items" }

// KnowledgeItemRow RAG 回查结果 DTO
type KnowledgeItemRow struct {
	VectorID string
	Title    string
	Content  string
	Category string
}
