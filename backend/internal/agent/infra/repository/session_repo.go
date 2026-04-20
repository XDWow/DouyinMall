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

type SessionRoundAsyncPublisher interface {
	PublishRound(ctx context.Context, in domain.RoundPersistInput, userMessage, assistantMessage domain.SessionMessage) error
}

type sessionRepository struct {
	db              *gorm.DB
	cache           agentcache.SessionCache
	roundAsyncQueue SessionRoundAsyncPublisher
}

func NewSessionRepository(db *gorm.DB, cache agentcache.SessionCache, roundAsync SessionRoundAsyncPublisher) domainrepo.SessionRepository {
	return &sessionRepository{db: db, cache: cache, roundAsyncQueue: roundAsync}
}

func (s *sessionRepository) Load(ctx context.Context, sessionID string) (*domain.LoadedSession, []domain.SessionMessage, error) {
	if sessionID == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}

	if loaded, messages, err := s.loadFromCache(ctx, sessionID); err == nil && loaded != nil {
		return loaded, messages, nil
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
	loaded := sessionRowToLoaded(&row)
	_ = s.saveToCache(ctx, loaded, messages)
	return loaded, messages, nil
}

func (s *sessionRepository) Create(ctx context.Context, user domain.Session, meta domain.SessionTableMeta, slots map[string]any) error {
	row := domainSessionToModel(user, meta, slots)
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	return s.saveToCache(ctx, &domain.LoadedSession{User: user, Meta: meta, Slots: slots}, nil)
}

func (s *sessionRepository) Clear(ctx context.Context, sessionID string) error {
	_ = s.deleteFromCache(ctx, sessionID)

	if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&agentdb.Message{}).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&agentdb.Session{}).Error
}

func (s *sessionRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.SessionListItem, int, error) {
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

	out := make([]domain.SessionListItem, 0, len(rows))
	for i := range rows {
		ld := sessionRowToLoaded(&rows[i])
		out = append(out, domain.SessionListItem{Context: ld.User, Meta: ld.Meta})
	}
	return out, int(total), nil
}

func (s *sessionRepository) loadFromCache(ctx context.Context, sessionID string) (*domain.LoadedSession, []domain.SessionMessage, error) {
	if s.cache == nil {
		return nil, nil, nil
	}
	return s.cache.Load(ctx, sessionID)
}

func (s *sessionRepository) saveToCache(ctx context.Context, loaded *domain.LoadedSession, messages []domain.SessionMessage) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Save(ctx, loaded, messages)
}

func (s *sessionRepository) deleteFromCache(ctx context.Context, sessionID string) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Delete(ctx, sessionID)
}

func (s *sessionRepository) SaveRound(ctx context.Context, in domain.RoundPersistInput, userMessage domain.SessionMessage, assistantMessage domain.SessionMessage) error {
	messages := make([]domain.SessionMessage, 0, 2)
	if userMessage.Content != "" {
		messages = append(messages, userMessage)
	}
	if assistantMessage.Content != "" {
		messages = append(messages, assistantMessage)
	}
	return s.saveMessages(ctx, in.User.SessionID, messages, &in)
}

func (s *sessionRepository) SaveRoundPersistent(ctx context.Context, in domain.RoundPersistInput, userMessage domain.SessionMessage, assistantMessage domain.SessionMessage) error {
	if s.roundAsyncQueue != nil {
		return s.roundAsyncQueue.PublishRound(ctx, in, userMessage, assistantMessage)
	}
	messages := make([]domain.SessionMessage, 0, 2)
	if userMessage.Content != "" {
		messages = append(messages, userMessage)
	}
	if assistantMessage.Content != "" {
		messages = append(messages, assistantMessage)
	}
	return s.persistMessages(ctx, in.User.SessionID, messages, &in)
}

func (s *sessionRepository) SaveCacheSnapshot(ctx context.Context, in domain.RoundPersistInput, messages []domain.SessionMessage) error {
	return s.saveToCache(ctx, &domain.LoadedSession{User: in.User, Meta: in.Meta, Slots: in.Slots}, messages)
}

func (s *sessionRepository) SaveMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage) error {
	return s.saveMessages(ctx, sessionID, messages, nil)
}

func (s *sessionRepository) saveMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage, in *domain.RoundPersistInput) error {
	if err := s.persistMessages(ctx, sessionID, messages, in); err != nil {
		return err
	}
	allMessages, err := s.LoadAllMessages(ctx, sessionID)
	if err != nil {
		return err
	}
	if in == nil {
		var row agentdb.Session
		if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&row).Error; err != nil {
			return err
		}
		loaded := sessionRowToLoaded(&row)
		return s.saveToCache(ctx, loaded, allMessages)
	}
	return s.saveToCache(ctx, &domain.LoadedSession{User: in.User, Meta: in.Meta, Slots: in.Slots}, allMessages)
}

func (s *sessionRepository) persistMessages(ctx context.Context, sessionID string, messages []domain.SessionMessage, in *domain.RoundPersistInput) error {
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
	if in != nil {
		now := time.Now()
		slotsJSON := marshalSlots(domain.PackUserSessionIntoSlots(in.Slots, in.User))
		if err := s.db.WithContext(ctx).Model(&agentdb.Session{}).
			Where("session_id = ?", sessionID).
			Updates(map[string]any{
				"status":       string(in.Meta.Status),
				"last_message": in.Meta.LastMessage,
				"total_turns":  in.Meta.TotalTurns,
				"slots_json":   slotsJSON,
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

func domainSessionToModel(user domain.Session, meta domain.SessionTableMeta, slots map[string]any) *agentdb.Session {
	status := string(meta.Status)
	if status == "" {
		status = string(domain.SessionStatusActive)
	}
	return &agentdb.Session{
		SessionID:   user.SessionID,
		UserID:      user.UserID,
		Status:      status,
		LastMessage: meta.LastMessage,
		TotalTurns:  meta.TotalTurns,
		SlotsJSON:   marshalSlots(domain.PackUserSessionIntoSlots(slots, user)),
		CreatedAt:   meta.CreatedAt,
		UpdatedAt:   meta.UpdatedAt,
	}
}

func sessionRowToLoaded(row *agentdb.Session) *domain.LoadedSession {
	if row == nil {
		return nil
	}
	full := unmarshalSlots(row.SlotsJSON)
	user, toolSlots := domain.UnpackUserSessionFromSlots(full)
	user.SessionID = row.SessionID
	user.UserID = row.UserID
	meta := domain.SessionTableMeta{
		Status:      domain.SessionStatus(row.Status),
		LastMessage: row.LastMessage,
		TotalTurns:  row.TotalTurns,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	return &domain.LoadedSession{User: user, Meta: meta, Slots: toolSlots}
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
