//go:build legacy_agent

package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	"gorm.io/gorm"
)

// ==================== GORM Models ====================

type SessionDO struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID  string    `gorm:"uniqueIndex;type:varchar(64);not null"`
	UserID     uint64    `gorm:"index;not null"`
	Channel    string    `gorm:"type:varchar(32);not null;default:web"`
	Status     uint8     `gorm:"not null;default:1"`
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
	Intent     int8      `gorm:"not null;default:0"`
	Confidence float32   `gorm:"not null;default:0"`
	TokensUsed int       `gorm:"not null;default:0"`
	LatencyMs  int       `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"index:idx_session_created;autoCreateTime"`
}

func (MessageDO) TableName() string { return "agent_messages" }

type KnowledgeItemDO struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	Title     string    `gorm:"type:varchar(256);not null"`
	Content   string    `gorm:"type:text;not null"`
	Category  string    `gorm:"index;type:varchar(64);not null"`
	VectorID  string    `gorm:"type:varchar(64);default:''"`
	Status    int8      `gorm:"index;not null;default:1"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (KnowledgeItemDO) TableName() string { return "knowledge_items" }

// ==================== DAO ====================

type AgentDAO struct {
	db *gorm.DB
}

func NewAgentDAO(db *gorm.DB) *AgentDAO {
	return &AgentDAO{db: db}
}

// InitTables 鑷姩寤鸿〃
func (d *AgentDAO) InitTables() error {
	return d.db.AutoMigrate(&SessionDO{}, &MessageDO{}, &KnowledgeItemDO{})
}

// ---- Session ----

func (d *AgentDAO) CreateSession(ctx context.Context, session *SessionDO) error {
	return d.db.WithContext(ctx).Create(session).Error
}

func (d *AgentDAO) GetSession(ctx context.Context, sessionID string) (*SessionDO, error) {
	var s SessionDO
	err := d.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *AgentDAO) UpdateSession(ctx context.Context, session *SessionDO) error {
	return d.db.WithContext(ctx).Where("session_id = ?", session.SessionID).
		Updates(map[string]any{
			"status":      session.Status,
			"summary":     session.Summary,
			"total_turns": session.TotalTurns,
		}).Error
}

func (d *AgentDAO) ListSessionsByUser(ctx context.Context, userID uint64, limit, offset int) ([]SessionDO, int64, error) {
	var sessions []SessionDO
	var total int64
	query := d.db.WithContext(ctx).Where("user_id = ?", userID)
	if err := query.Model(&SessionDO{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&sessions).Error
	return sessions, total, err
}

func (d *AgentDAO) DeleteSession(ctx context.Context, sessionID string) error {
	return d.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&SessionDO{}).Error
}

// ---- Message ----

func (d *AgentDAO) CreateMessage(ctx context.Context, msg *MessageDO) error {
	return d.db.WithContext(ctx).Create(msg).Error
}

func (d *AgentDAO) BatchCreateMessages(ctx context.Context, msgs []MessageDO) error {
	if len(msgs) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).Create(&msgs).Error
}

func (d *AgentDAO) GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]MessageDO, int64, error) {
	var msgs []MessageDO
	var total int64
	query := d.db.WithContext(ctx).Where("session_id = ?", sessionID)
	if err := query.Model(&MessageDO{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at ASC").Limit(limit).Offset(offset).Find(&msgs).Error
	return msgs, total, err
}

func (d *AgentDAO) DeleteMessages(ctx context.Context, sessionID string) error {
	return d.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&MessageDO{}).Error
}

// ==================== Domain 杞崲 ====================

func ToDomainSession(do *SessionDO) *domain.Session {
	return &domain.Session{
		ID:         do.SessionID,
		UserID:     int64(do.UserID),
		Channel:    do.Channel,
		Status:     domain.SessionStatus(do.Status),
		Summary:    do.Summary,
		TotalTurns: do.TotalTurns,
		CreatedAt:  do.CreatedAt,
		UpdatedAt:  do.UpdatedAt,
	}
}

func ToSessionDO(s *domain.Session) *SessionDO {
	return &SessionDO{
		SessionID:  s.ID,
		UserID:     uint64(s.UserID),
		Channel:    s.Channel,
		Status:     uint8(s.Status),
		Summary:    s.Summary,
		TotalTurns: s.TotalTurns,
	}
}

func ToDomainMessage(do *MessageDO) domain.Message {
	return domain.Message{
		ID:         fmt.Sprintf("%d", do.ID),
		SessionID:  do.SessionID,
		Role:       domain.Role(do.Role),
		Content:    do.Content,
		Intent:     domain.IntentType(do.Intent),
		Confidence: do.Confidence,
		TokensUsed: do.TokensUsed,
		LatencyMs:  int64(do.LatencyMs),
		CreatedAt:  do.CreatedAt,
	}
}

func ToMessageDO(m domain.Message) *MessageDO {
	return &MessageDO{
		SessionID:  m.SessionID,
		Role:       string(m.Role),
		Content:    m.Content,
		Intent:     int8(m.Intent),
		Confidence: m.Confidence,
		TokensUsed: m.TokensUsed,
		LatencyMs:  int(m.LatencyMs),
	}
}
