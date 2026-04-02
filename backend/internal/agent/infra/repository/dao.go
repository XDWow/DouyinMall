package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type SessionDO struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID  string    `gorm:"uniqueIndex;type:varchar(64);not null"`
	UserID     int64     `gorm:"index;not null"`
	Channel    string    `gorm:"type:varchar(32);not null"`
	Status     string    `gorm:"type:varchar(16);not null"`
	Summary    string    `gorm:"type:text"`
	TotalTurns int       `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

func (SessionDO) TableName() string { return "agent_sessions" }

type MessageDO struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID  string    `gorm:"index:idx_session_created;type:varchar(64);not null"`
	Role       string    `gorm:"type:varchar(16);not null"`
	Content    string    `gorm:"type:text;not null"`
	Intent     string    `gorm:"type:varchar(32);not null"`
	Confidence float64   `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"index:idx_session_created;autoCreateTime"`
}

func (MessageDO) TableName() string { return "agent_messages" }

type KnowledgeChunkDO struct {
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

func (KnowledgeChunkDO) TableName() string { return "agent_knowledge_chunks" }

type DAO struct {
	db *gorm.DB
}

func NewDAO(db *gorm.DB) *DAO {
	return &DAO{db: db}
}

func (d *DAO) InitTables(ctx context.Context) error {
	return d.db.WithContext(ctx).AutoMigrate(&SessionDO{}, &MessageDO{}, &KnowledgeChunkDO{})
}
