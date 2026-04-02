package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
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

func NewSessionStore(dao *DAO, rdb redis.Cmdable) memory.Store {
	return &SessionStore{dao: dao, rdb: rdb}
}

func (s *SessionStore) Load(ctx context.Context, sessionID string) (*memory.Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	if session, err := s.loadFromRedis(ctx, sessionID); err == nil && session != nil {
		return session, nil
	}

	var sessionDO SessionDO
	if err := s.dao.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&sessionDO).Error; err != nil {
		return nil, err
	}

	var messageDOs []MessageDO
	if err := s.dao.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Limit(20).
		Find(&messageDOs).Error; err != nil {
		return nil, err
	}

	session := toMemorySession(&sessionDO, messageDOs)
	_ = s.saveToRedis(ctx, session)
	return session, nil
}

func (s *SessionStore) Create(ctx context.Context, session *memory.Session) error {
	if err := s.dao.db.WithContext(ctx).Create(toSessionDO(session)).Error; err != nil {
		return err
	}
	return s.saveToRedis(ctx, session)
}

func (s *SessionStore) Save(ctx context.Context, session *memory.Session) error {
	if session == nil {
		return nil
	}

	if err := s.saveToRedis(ctx, session); err != nil {
		return err
	}

	if err := s.dao.db.WithContext(ctx).
		Where("session_id = ?", session.ID).
		Updates(map[string]any{
			"status":      string(session.Status),
			"summary":     session.Summary,
			"total_turns": session.TotalTurns,
			"updated_at":  time.Now(),
		}).Error; err != nil {
		return err
	}

	if len(session.Messages) == 0 {
		return nil
	}
	lastMessages := session.Messages
	if len(lastMessages) > 2 {
		lastMessages = lastMessages[len(lastMessages)-2:]
	}
	messageDOs := make([]MessageDO, 0, len(lastMessages))
	for _, message := range lastMessages {
		messageDOs = append(messageDOs, toMessageDO(message))
	}
	return s.dao.db.WithContext(ctx).Create(&messageDOs).Error
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

func (s *SessionStore) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]memory.Session, int, error) {
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

	out := make([]memory.Session, 0, len(sessionDOs))
	for _, item := range sessionDOs {
		out = append(out, memory.Session{
			ID:         item.SessionID,
			UserID:     item.UserID,
			Channel:    item.Channel,
			Status:     dto.SessionStatus(item.Status),
			Summary:    item.Summary,
			TotalTurns: item.TotalTurns,
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return out, int(total), nil
}

func (s *SessionStore) loadFromRedis(ctx context.Context, sessionID string) (*memory.Session, error) {
	raw, err := s.rdb.Get(ctx, sessionMetaKeyPrefix+sessionID).Bytes()
	if err != nil {
		return nil, err
	}
	var session memory.Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, err
	}

	messageRaws, err := s.rdb.LRange(ctx, sessionMetaKeyPrefix+sessionID+sessionMsgKeySuffix, 0, -1).Result()
	if err == nil {
		session.Messages = make([]dto.Message, 0, len(messageRaws))
		for _, rawMessage := range messageRaws {
			var message dto.Message
			if json.Unmarshal([]byte(rawMessage), &message) == nil {
				session.Messages = append(session.Messages, message)
			}
		}
	}
	return &session, nil
}

func (s *SessionStore) saveToRedis(ctx context.Context, session *memory.Session) error {
	meta, err := json.Marshal(session)
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, sessionMetaKeyPrefix+session.ID, meta, sessionTTL)
	if len(session.Messages) > 0 {
		items := make([]any, 0, len(session.Messages))
		recent := session.Messages
		if len(recent) > 20 {
			recent = recent[len(recent)-20:]
		}
		for _, message := range recent {
			raw, err := json.Marshal(message)
			if err != nil {
				continue
			}
			items = append(items, raw)
		}
		if len(items) > 0 {
			msgKey := sessionMetaKeyPrefix + session.ID + sessionMsgKeySuffix
			pipe.Del(ctx, msgKey)
			pipe.RPush(ctx, msgKey, items...)
			pipe.Expire(ctx, msgKey, sessionTTL)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

func toMemorySession(session *SessionDO, messages []MessageDO) *memory.Session {
	out := &memory.Session{
		ID:         session.SessionID,
		UserID:     session.UserID,
		Channel:    session.Channel,
		Status:     dto.SessionStatus(session.Status),
		Summary:    session.Summary,
		TotalTurns: session.TotalTurns,
		CreatedAt:  session.CreatedAt,
		UpdatedAt:  session.UpdatedAt,
		Messages:   make([]dto.Message, 0, len(messages)),
	}
	for _, message := range messages {
		out.Messages = append(out.Messages, dto.Message{
			ID:         fmt.Sprintf("%d", message.ID),
			SessionID:  message.SessionID,
			Role:       dto.Role(message.Role),
			Content:    message.Content,
			Intent:     dto.Intent(message.Intent),
			Confidence: message.Confidence,
			CreatedAt:  message.CreatedAt,
		})
	}
	return out
}

func toSessionDO(session *memory.Session) *SessionDO {
	return &SessionDO{
		SessionID:  session.ID,
		UserID:     session.UserID,
		Channel:    session.Channel,
		Status:     string(session.Status),
		Summary:    session.Summary,
		TotalTurns: session.TotalTurns,
	}
}

func toMessageDO(message dto.Message) MessageDO {
	return MessageDO{
		SessionID:  message.SessionID,
		Role:       string(message.Role),
		Content:    message.Content,
		Intent:     string(message.Intent),
		Confidence: message.Confidence,
		CreatedAt:  message.CreatedAt,
	}
}
