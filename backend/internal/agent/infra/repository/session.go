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
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

type SessionStore struct {
	dao   *DAO
	cache agentcache.SessionCache
}

func NewSessionStore(dao *DAO, cache agentcache.SessionCache) *SessionStore {
	return &SessionStore{dao: dao, cache: cache}
}

func (s *SessionStore) Load(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error) {
	if sessionID == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}

	if meta, messages, err := s.loadFromCache(ctx, sessionID); err == nil && meta != nil {
		return meta, messages, nil
	}

	var sessionDO SessionDO
	if err := s.dao.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&sessionDO).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	messages, err := s.LoadAllMessages(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	meta := toDomainSession(&sessionDO)
	_ = s.saveToCache(ctx, meta, messages)
	return &meta, messages, nil
}

func (s *SessionStore) Create(ctx context.Context, session domain.Session) error {
	if err := s.dao.db.WithContext(ctx).Create(toSessionDO(session)).Error; err != nil {
		return err
	}
	return s.saveToCache(ctx, session, nil)
}

func (s *SessionStore) Clear(ctx context.Context, sessionID string) error {
	_ = s.deleteFromCache(ctx, sessionID)

	if err := s.dao.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&MessageDO{}).Error; err != nil {
		return err
	}
	return s.dao.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&SessionDO{}).Error
}

func (s *SessionStore) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	if limit <= 0 {
		limit = 10
	}

	var total int64
	query := s.dao.db.WithContext(ctx).Model(&SessionDO{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var sessionDOs []SessionDO
	if err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&sessionDOs).Error; err != nil {
		return nil, 0, err
	}

	out := make([]domain.Session, 0, len(sessionDOs))
	for _, item := range sessionDOs {
		out = append(out, toDomainSession(&item))
	}
	return out, int(total), nil
}

func (s *SessionStore) loadFromCache(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error) {
	if s.cache == nil {
		return nil, nil, nil
	}
	return s.cache.Load(ctx, sessionID)
}

func (s *SessionStore) saveToCache(ctx context.Context, session domain.Session, messages []domain.Message) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Save(ctx, session, messages)
}

func (s *SessionStore) deleteFromCache(ctx context.Context, sessionID string) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Delete(ctx, sessionID)
}

func (s *SessionStore) SaveRound(ctx context.Context, session domain.Session, userMessage domain.Message, assistantMessage domain.Message) error {
	messages := make([]domain.Message, 0, 2)
	if userMessage.Content != "" {
		messages = append(messages, userMessage)
	}
	if assistantMessage.Content != "" {
		messages = append(messages, assistantMessage)
	}
	return s.saveMessages(ctx, session.SessionID, messages, &session)
}

func (s *SessionStore) SaveRoundPersistent(ctx context.Context, session domain.Session, userMessage domain.Message, assistantMessage domain.Message) error {
	messages := make([]domain.Message, 0, 2)
	if userMessage.Content != "" {
		messages = append(messages, userMessage)
	}
	if assistantMessage.Content != "" {
		messages = append(messages, assistantMessage)
	}
	return s.persistMessages(ctx, session.SessionID, messages, &session)
}

func (s *SessionStore) SaveCacheSnapshot(ctx context.Context, session domain.Session, messages []domain.Message) error {
	return s.saveToCache(ctx, session, messages)
}

func (s *SessionStore) SaveMessages(ctx context.Context, sessionID string, messages []domain.Message) error {
	return s.saveMessages(ctx, sessionID, messages, nil)
}

func (s *SessionStore) saveMessages(ctx context.Context, sessionID string, messages []domain.Message, session *domain.Session) error {
	if err := s.persistMessages(ctx, sessionID, messages, session); err != nil {
		return err
	}
	allMessages, err := s.LoadAllMessages(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		var sessionDO SessionDO
		if err := s.dao.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&sessionDO).Error; err != nil {
			return err
		}
		domainSession := toDomainSession(&sessionDO)
		return s.saveToCache(ctx, domainSession, allMessages)
	}
	return s.saveToCache(ctx, *session, allMessages)
}

func (s *SessionStore) persistMessages(ctx context.Context, sessionID string, messages []domain.Message, session *domain.Session) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	messageDOs := make([]MessageDO, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		messageDOs = append(messageDOs, toMessageDO(message))
	}
	if len(messageDOs) > 0 {
		if err := s.dao.db.WithContext(ctx).Create(&messageDOs).Error; err != nil {
			return err
		}
	}
	if session != nil {
		now := time.Now()
		if err := s.dao.db.WithContext(ctx).Model(&SessionDO{}).
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

func (s *SessionStore) LoadAllMessages(ctx context.Context, sessionID string) ([]domain.Message, error) {
	var messageDOs []MessageDO
	if err := s.dao.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&messageDOs).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Message, 0, len(messageDOs))
	for _, message := range messageDOs {
		out = append(out, domain.Message{
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

func toSessionDO(session domain.Session) *SessionDO {
	return &SessionDO{
		SessionID:   session.SessionID,
		UserID:      session.UserID,
		Status:      string(session.Status),
		LastMessage: session.LastMessage,
		TotalTurns:  session.TotalTurns,
		SlotsJSON:   marshalSlots(session.Slots),
	}
}

func toDomainSession(session *SessionDO) domain.Session {
	return domain.Session{
		SessionID:   session.SessionID,
		UserID:      session.UserID,
		Status:      domain.SessionStatus(session.Status),
		LastMessage: session.LastMessage,
		TotalTurns:  session.TotalTurns,
		Slots:       unmarshalSlots(session.SlotsJSON),
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}
}

func toMessageDO(message domain.Message) MessageDO {
	return MessageDO{
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
