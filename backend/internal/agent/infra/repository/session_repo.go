package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	domainrepo "github.com/XDWow/DouyinMall/backend/internal/agent/domain/repo"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentdb "github.com/XDWow/DouyinMall/backend/internal/agent/infra/db"
)

type sessionRepository struct {
	db    *gorm.DB
	cache agentcache.SessionCache
}

// NewSessionRepository 会话仓储：直接持 *gorm.DB，表模型见 infra/db/model.go。
func NewSessionRepository(db *gorm.DB, cache agentcache.SessionCache) domainrepo.SessionRepository {
	return &sessionRepository{db: db, cache: cache}
}

func (s *sessionRepository) Load(ctx context.Context, sessionID string) (*domain.Session, []domain.SessionMessage, error) {
	if sessionID == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}

	if meta, messages, err := s.loadFromCache(ctx, sessionID); err == nil && meta != nil {
		return meta, messages, nil
	}

	var row agentdb.Session
	if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	messages, err := s.LoadAllMessages(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	meta := sessionRowToDomain(&row)
	_ = s.saveToCache(ctx, meta, messages)
	return &meta, messages, nil
}

func (s *sessionRepository) Create(ctx context.Context, session domain.Session) error {
	if err := s.db.WithContext(ctx).Create(domainSessionToModel(session)).Error; err != nil {
		return err
	}
	return s.saveToCache(ctx, session, nil)
}

func (s *sessionRepository) Clear(ctx context.Context, sessionID string) error {
	_ = s.deleteFromCache(ctx, sessionID)

	if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&agentdb.Message{}).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&agentdb.Session{}).Error
}

func (s *sessionRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	if limit <= 0 {
		limit = 10
	}

	var total int64
	query := s.db.WithContext(ctx).Model(&agentdb.Session{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []agentdb.Session
	if err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]domain.Session, 0, len(rows))
	for i := range rows {
		out = append(out, sessionRowToDomain(&rows[i]))
	}
	return out, int(total), nil
}

func (s *sessionRepository) loadFromCache(ctx context.Context, sessionID string) (*domain.Session, []domain.SessionMessage, error) {
	if s.cache == nil {
		return nil, nil, nil
	}
	return s.cache.Load(ctx, sessionID)
}

func (s *sessionRepository) saveToCache(ctx context.Context, session domain.Session, messages []domain.SessionMessage) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Save(ctx, session, messages)
}

func (s *sessionRepository) deleteFromCache(ctx context.Context, sessionID string) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Delete(ctx, sessionID)
}

func (s *sessionRepository) SaveRound(ctx context.Context, session domain.Session, userMessage domain.SessionMessage, assistantMessage domain.SessionMessage) error {
	messages := make([]domain.SessionMessage, 0, 2)
	if userMessage.Content != "" {
		messages = append(messages, userMessage)
	}
	if assistantMessage.Content != "" {
		messages = append(messages, assistantMessage)
	}
	return s.saveMessages(ctx, session.SessionID, messages, &session)
}

func (s *sessionRepository) SaveRoundPersistent(ctx context.Context, session domain.Session, userMessage domain.SessionMessage, assistantMessage domain.SessionMessage) error {
	messages := make([]domain.SessionMessage, 0, 2)
	if userMessage.Content != "" {
		messages = append(messages, userMessage)
	}
	if assistantMessage.Content != "" {
		messages = append(messages, assistantMessage)
	}
	return s.persistMessages(ctx, session.SessionID, messages, &session)
}

func (s *sessionRepository) SaveCacheSnapshot(ctx context.Context, session domain.Session, messages []domain.SessionMessage) error {
	return s.saveToCache(ctx, session, messages)
}

func (s *sessionRepository) SaveMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage) error {
	return s.saveMessages(ctx, sessionID, messages, nil)
}

func (s *sessionRepository) saveMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage, session *domain.Session) error {
	if err := s.persistMessages(ctx, sessionID, messages, session); err != nil {
		return err
	}
	allMessages, err := s.LoadAllMessages(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		var row agentdb.Session
		if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&row).Error; err != nil {
			return err
		}
		domainSession := sessionRowToDomain(&row)
		return s.saveToCache(ctx, domainSession, allMessages)
	}
	return s.saveToCache(ctx, *session, allMessages)
}

func (s *sessionRepository) persistMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage, session *domain.Session) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	dbMsgs := make([]agentdb.Message, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		dbMsgs = append(dbMsgs, domainMessageToModel(message))
	}
	if len(dbMsgs) > 0 {
		if err := s.db.WithContext(ctx).Create(&dbMsgs).Error; err != nil {
			return err
		}
	}
	if session != nil {
		now := time.Now()
		if err := s.db.WithContext(ctx).Model(&agentdb.Session{}).
			Where("session_id = ?", sessionID).
			Updates(map[string]any{
				"status":       string(session.Status),
				"last_message": session.LastMessage,
				"total_turns":  session.TotalTurns,
				"slots_json":   marshalSlots(session.Slots),
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *sessionRepository) LoadAllMessages(ctx context.Context, sessionID string) ([]domain.SessionMessage, error) {
	var rows []agentdb.Message
	if err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.SessionMessage, 0, len(rows))
	for _, message := range rows {
		out = append(out, domain.SessionMessage{
			ID:         fmt.Sprintf("%d", message.ID),
			SessionID:  message.SessionID,
			Role:       domain.Role(message.Role),
			Content:    message.Content,
			Intent:     domain.Intent(message.Intent),
			Confidence: message.Confidence,
			CreatedAt:  message.CreatedAt,
		})
	}
	return out, nil
}

func domainSessionToModel(session domain.Session) *agentdb.Session {
	return &agentdb.Session{
		SessionID:   session.SessionID,
		UserID:      session.UserID,
		Status:      string(session.Status),
		LastMessage: session.LastMessage,
		TotalTurns:  session.TotalTurns,
		SlotsJSON:   marshalSlots(session.Slots),
	}
}

func sessionRowToDomain(row *agentdb.Session) domain.Session {
	return domain.Session{
		SessionID:   row.SessionID,
		UserID:      row.UserID,
		Status:      domain.SessionStatus(row.Status),
		LastMessage: row.LastMessage,
		TotalTurns:  row.TotalTurns,
		Slots:       unmarshalSlots(row.SlotsJSON),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func domainMessageToModel(message domain.SessionMessage) agentdb.Message {
	return agentdb.Message{
		SessionID:  message.SessionID,
		Role:       string(message.Role),
		Content:    message.Content,
		Intent:     string(message.Intent),
		Confidence: message.Confidence,
		CreatedAt:  message.CreatedAt,
	}
}

// marshalSlots 把会话槽位序列化到 session 表，便于跨轮次恢复。
func marshalSlots(slots map[string]any) string {
	if len(slots) == 0 {
		return ""
	}
	raw, err := json.Marshal(slots)
	if err != nil {
		return ""
	}
	return string(raw)
}

func unmarshalSlots(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var slots map[string]any
	if err := json.Unmarshal([]byte(raw), &slots); err != nil || len(slots) == 0 {
		return nil
	}
	return slots
}
