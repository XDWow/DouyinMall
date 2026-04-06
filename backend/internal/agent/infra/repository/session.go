package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

const (
	sessionMetaKeyPrefix = "agent:session:"
	sessionMsgKeySuffix  = ":msgs"
	sessionTTL           = 24 * time.Hour
)

type SessionStore struct {
	dao *DAO
	rdb redis.Cmdable
}

func NewSessionStore(dao *DAO, rdb redis.Cmdable) *SessionStore {
	return &SessionStore{dao: dao, rdb: rdb}
}

func (s *SessionStore) Load(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error) {
	if sessionID == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}

	if meta, messages, err := s.loadFromRedis(ctx, sessionID); err == nil && meta != nil {
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
	_ = s.saveToRedis(ctx, meta, messages)
	return &meta, messages, nil
}

func (s *SessionStore) Create(ctx context.Context, session domain.Session) error {
	if err := s.dao.db.WithContext(ctx).Create(toSessionDO(session)).Error; err != nil {
		return err
	}
	return s.saveToRedis(ctx, session, nil)
}

func (s *SessionStore) Clear(ctx context.Context, sessionID string) error {
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, sessionMetaKeyPrefix+sessionID, sessionMetaKeyPrefix+sessionID+sessionMsgKeySuffix)
	_, _ = pipe.Exec(ctx)

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

func (s *SessionStore) loadFromRedis(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error) {
	raw, err := s.rdb.Get(ctx, sessionMetaKeyPrefix+sessionID).Bytes()
	if err != nil {
		return nil, nil, err
	}
	var session domain.Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, nil, err
	}

	messages := []domain.Message(nil)
	messageRaws, err := s.rdb.LRange(ctx, sessionMetaKeyPrefix+sessionID+sessionMsgKeySuffix, 0, -1).Result()
	if err == nil {
		messages = make([]domain.Message, 0, len(messageRaws))
		for _, rawMessage := range messageRaws {
			var message domain.Message
			if json.Unmarshal([]byte(rawMessage), &message) == nil {
				messages = append(messages, message)
			}
		}
	}
	return &session, messages, nil
}

func (s *SessionStore) saveToRedis(ctx context.Context, session domain.Session, messages []domain.Message) error {
	meta, err := json.Marshal(session)
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, sessionMetaKeyPrefix+session.SessionID, meta, sessionTTL)
	if len(messages) > 0 {
		items := make([]any, 0, len(messages))
		recent := messages
		size := 10
		if len(recent) > size {
			recent = recent[len(recent)-size:]
		}
		for _, message := range recent {
			raw, err := json.Marshal(message)
			if err != nil {
				continue
			}
			items = append(items, raw)
		}
		if len(items) > 0 {
			msgKey := sessionMetaKeyPrefix + session.SessionID + sessionMsgKeySuffix
			pipe.Del(ctx, msgKey)
			pipe.RPush(ctx, msgKey, items...)
			pipe.Expire(ctx, msgKey, sessionTTL)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
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

func (s *SessionStore) SaveMessages(ctx context.Context, sessionID string, messages []domain.Message) error {
	return s.saveMessages(ctx, sessionID, messages, nil)
}

func (s *SessionStore) saveMessages(ctx context.Context, sessionID string, messages []domain.Message, session *domain.Session) error {
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
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}
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
		return s.saveToRedis(ctx, domainSession, allMessages)
	}
	return s.saveToRedis(ctx, *session, allMessages)
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
	}
}

func toDomainSession(session *SessionDO) domain.Session {
	return domain.Session{
		SessionID:   session.SessionID,
		UserID:      session.UserID,
		Status:      domain.SessionStatus(session.Status),
		LastMessage: session.LastMessage,
		TotalTurns:  session.TotalTurns,
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

