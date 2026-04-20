package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

const (
	sessionMetaKeyPrefix = "agent:session:"
	sessionMsgKeySuffix  = ":msgs"
)

type sessionCacheEnvelope struct {
	User  domain.Session           `json:"user"`
	Meta  domain.SessionTableMeta `json:"meta"`
	Slots map[string]any          `json:"slots,omitempty"`
}

type SessionCache interface {
	Load(ctx context.Context, sessionID string) (*domain.LoadedSession, []domain.SessionMessage, error)
	Save(ctx context.Context, loaded *domain.LoadedSession, messages []domain.SessionMessage) error
	Delete(ctx context.Context, sessionID string) error
}

type RedisSessionCache struct {
	store         Store
	ttl           time.Duration
	messageWindow int
}

func NewRedisSessionCache(store Store, ttl time.Duration, messageWindow int) *RedisSessionCache {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if messageWindow <= 0 {
		messageWindow = 10
	}
	return &RedisSessionCache{
		store:         store,
		ttl:           ttl,
		messageWindow: messageWindow,
	}
}

func (c *RedisSessionCache) Load(ctx context.Context, sessionID string) (*domain.LoadedSession, []domain.SessionMessage, error) {
	if c == nil || c.store == nil {
		return nil, nil, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}

	raw, err := c.store.Get(ctx, sessionMetaKeyPrefix+sessionID)
	if err != nil || len(raw) == 0 {
		return nil, nil, err
	}

	var env sessionCacheEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, err
	}
	loaded := &domain.LoadedSession{
		User:  env.User,
		Meta:  env.Meta,
		Slots: env.Slots,
	}

	rawMessages, err := c.store.ListRange(ctx, sessionMetaKeyPrefix+sessionID+sessionMsgKeySuffix, 0, -1)
	if err != nil {
		return loaded, nil, err
	}

	messages := make([]domain.SessionMessage, 0, len(rawMessages))
	for _, item := range rawMessages {
		var message domain.SessionMessage
		if err := json.Unmarshal(item, &message); err == nil {
			messages = append(messages, message)
		}
	}
	return loaded, messages, nil
}

func (c *RedisSessionCache) Save(ctx context.Context, loaded *domain.LoadedSession, messages []domain.SessionMessage) error {
	if c == nil || c.store == nil {
		return nil
	}
	if loaded == nil || strings.TrimSpace(loaded.User.SessionID) == "" {
		return fmt.Errorf("session_id is required")
	}

	env := sessionCacheEnvelope{User: loaded.User, Meta: loaded.Meta, Slots: loaded.Slots}
	meta, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if err := c.store.Set(ctx, sessionMetaKeyPrefix+loaded.User.SessionID, meta, c.ttl); err != nil {
		return err
	}

	if len(messages) == 0 {
		return nil
	}

	recent := messages
	if len(recent) > c.messageWindow {
		recent = recent[len(recent)-c.messageWindow:]
	}

	items := make([][]byte, 0, len(recent))
	for _, message := range recent {
		raw, err := json.Marshal(message)
		if err != nil {
			continue
		}
		items = append(items, raw)
	}
	if len(items) == 0 {
		return nil
	}

	return c.store.ReplaceList(ctx, sessionMetaKeyPrefix+loaded.User.SessionID+sessionMsgKeySuffix, items, c.ttl)
}

func (c *RedisSessionCache) Delete(ctx context.Context, sessionID string) error {
	if c == nil || c.store == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	return c.store.Delete(ctx, sessionMetaKeyPrefix+sessionID, sessionMetaKeyPrefix+sessionID+sessionMsgKeySuffix)
}
