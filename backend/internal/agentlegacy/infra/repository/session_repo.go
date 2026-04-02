//go:build legacy_agent

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/infra/persistence"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// SessionRepoImpl 缁勫悎 Redis 鐑眰 + MySQL 鍐峰眰锛屽疄鐜?domain.SessionRepo
type SessionRepoImpl struct {
	cache  *cache.RedisSessionCache
	dao    *persistence.AgentDAO
	logger logger.LoggerV1
}

func NewSessionRepo(
	cache *cache.RedisSessionCache,
	dao *persistence.AgentDAO,
	logger logger.LoggerV1,
) domain.SessionRepo {
	return &SessionRepoImpl{cache: cache, dao: dao, logger: logger}
}

// Load 浼樺厛 Redis锛宮iss 鍥炴簮 MySQL
func (r *SessionRepoImpl) Load(ctx context.Context, sessionID string) (*domain.Session, error) {
	// 1. 灏濊瘯 Redis
	session, err := r.cache.LoadSession(ctx, sessionID)
	if err == nil && session != nil {
		// 鍔犺浇娑堟伅绐楀彛
		msgs, _ := r.cache.LoadMessages(ctx, sessionID)
		session.Messages = msgs
		return session, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		r.logger.Warn("Redis load session 寮傚父锛屽洖婧?MySQL", logger.Error(err))
	}

	// 2. 鍥炴簮 MySQL
	sessionDO, dbErr := r.dao.GetSession(ctx, sessionID)
	if dbErr != nil {
		return nil, fmt.Errorf("session not found: %w", dbErr)
	}
	session = persistence.ToDomainSession(sessionDO)

	// 鍔犺浇娑堟伅
	msgDOs, _, _ := r.dao.GetMessages(ctx, sessionID, 20, 0)
	for _, m := range msgDOs {
		session.Messages = append(session.Messages, persistence.ToDomainMessage(&m))
	}

	// 3. 鍥炲～ Redis
	go func() {
		_ = r.cache.SaveSession(context.Background(), session)
		for _, msg := range session.Messages {
			_ = r.cache.AppendMessage(context.Background(), sessionID, msg)
		}
	}()

	return session, nil
}

// Save 鍏堝啓 Redis锛屽紓姝ヨ惤 MySQL
func (r *SessionRepoImpl) Save(ctx context.Context, session *domain.Session) error {
	// 鍐?Redis
	if err := r.cache.SaveSession(ctx, session); err != nil {
		r.logger.Error("Redis save session 澶辫触", logger.Error(err))
	}

	// 杩藉姞鏈€鏂扮殑娑堟伅鍒?Redis
	if len(session.Messages) > 0 {
		for _, msg := range session.Messages[len(session.Messages)-2:] { // 鏈€鍚庝袱鏉★細user + assistant
			_ = r.cache.AppendMessage(ctx, session.ID, msg)
		}
	}

	// 寮傛钀?MySQL
	go func() {
		sessionDO := persistence.ToSessionDO(session)
		if err := r.dao.UpdateSession(context.Background(), sessionDO); err != nil {
			r.logger.Error("MySQL update session 澶辫触", logger.Error(err))
		}
		// 钀芥秷鎭?		if len(session.Messages) >= 2 {
			msgs := session.Messages[len(session.Messages)-2:]
			dos := make([]persistence.MessageDO, len(msgs))
			for i, m := range msgs {
				dos[i] = *persistence.ToMessageDO(m)
			}
			if err := r.dao.BatchCreateMessages(context.Background(), dos); err != nil {
				r.logger.Error("MySQL batch create messages 澶辫触", logger.Error(err))
			}
		}
	}()

	return nil
}

// Create 鍒涘缓鏂颁細璇?func (r *SessionRepoImpl) Create(ctx context.Context, session *domain.Session) error {
	sessionDO := persistence.ToSessionDO(session)
	if err := r.dao.CreateSession(ctx, sessionDO); err != nil {
		return fmt.Errorf("mysql create session: %w", err)
	}
	return r.cache.SaveSession(ctx, session)
}

// Clear 娓呯┖浼氳瘽
func (r *SessionRepoImpl) Clear(ctx context.Context, sessionID string) error {
	_ = r.cache.DeleteSession(ctx, sessionID)
	_ = r.dao.DeleteMessages(ctx, sessionID)
	return nil
}

// ListByUser 鍒嗛〉鏌ヨ锛堣蛋 MySQL锛?func (r *SessionRepoImpl) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	dos, total, err := r.dao.ListSessionsByUser(ctx, uint64(userID), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	sessions := make([]domain.Session, len(dos))
	for i, do := range dos {
		sessions[i] = *persistence.ToDomainSession(&do)
	}
	return sessions, int(total), nil
}

// FindActiveByUser 鏌ユ壘鐢ㄦ埛娲昏穬浼氳瘽
func (r *SessionRepoImpl) FindActiveByUser(ctx context.Context, userID int64) (*domain.Session, error) {
	// TODO: 鍙粠 Redis agent:user:{uid}:active 蹇€熸煡鎵?	sessions, _, err := r.ListByUser(ctx, userID, 1, 0)
	if err != nil || len(sessions) == 0 {
		return nil, err
	}
	if sessions[0].Status == domain.SessionActive {
		return &sessions[0], nil
	}
	return nil, nil
}
