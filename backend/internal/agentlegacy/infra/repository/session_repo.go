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

// SessionRepoImpl 缂佸嫬鎮?Redis 閻戭厼鐪?+ MySQL 閸愬嘲鐪伴敍灞界杽閻?domain.SessionRepo
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

// Load 娴兼ê鍘?Redis閿涘iss 閸ョ偞绨?MySQL
func (r *SessionRepoImpl) Load(ctx context.Context, sessionID string) (*domain.Session, error) {
	// 1. 鐏忔繆鐦?Redis
	session, err := r.cache.LoadSession(ctx, sessionID)
	if err == nil && session != nil {
		// 閸旂姾娴囧☉鍫熶紖缁愭褰?		msgs, _ := r.cache.LoadMessages(ctx, sessionID)
		session.Messages = msgs
		return session, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		r.logger.Warn("Redis load session 瀵倸鐖堕敍灞芥礀濠?MySQL", logger.Error(err))
	}

	// 2. 閸ョ偞绨?MySQL
	sessionDO, dbErr := r.dao.GetSession(ctx, sessionID)
	if dbErr != nil {
		return nil, fmt.Errorf("session not found: %w", dbErr)
	}
	session = persistence.ToDomainSession(sessionDO)

	// 閸旂姾娴囧☉鍫熶紖
	msgDOs, _, _ := r.dao.GetMessages(ctx, sessionID, 20, 0)
	for _, m := range msgDOs {
		session.Messages = append(session.Messages, persistence.ToDomainMessage(&m))
	}

	// 3. 閸ョ偛锝?Redis
	go func() {
		_ = r.cache.SaveSession(context.Background(), session)
		for _, msg := range session.Messages {
			_ = r.cache.AppendMessage(context.Background(), sessionID, msg)
		}
	}()

	return session, nil
}

// Save 閸忓牆鍟?Redis閿涘苯绱撳銉ㄦ儰 MySQL
func (r *SessionRepoImpl) Save(ctx context.Context, session *domain.Session) error {
	// 閸?Redis
	if err := r.cache.SaveSession(ctx, session); err != nil {
		r.logger.Error("Redis save session 婢惰精瑙?, logger.Error(err))
	}

	// 鏉╄棄濮為張鈧弬鎵畱濞戝牊浼呴崚?Redis
	if len(session.Messages) > 0 {
		for _, msg := range session.Messages[len(session.Messages)-2:] { // 閺堚偓閸氬簼琚遍弶鈽呯窗user + assistant
			_ = r.cache.AppendMessage(ctx, session.ID, msg)
		}
	}

	// 瀵倹顒為拃?MySQL
	go func() {
		sessionDO := persistence.ToSessionDO(session)
		if err := r.dao.UpdateSession(context.Background(), sessionDO); err != nil {
			r.logger.Error("MySQL update session 婢惰精瑙?, logger.Error(err))
		}
		// 閽€鑺ョХ閹?		if len(session.Messages) >= 2 {
			msgs := session.Messages[len(session.Messages)-2:]
			dos := make([]persistence.MessageDO, len(msgs))
			for i, m := range msgs {
				dos[i] = *persistence.ToMessageDO(m)
			}
			if err := r.dao.BatchCreateMessages(context.Background(), dos); err != nil {
				r.logger.Error("MySQL batch create messages 婢惰精瑙?, logger.Error(err))
			}
		}
	}()

	return nil
}

// Create 閸掓稑缂撻弬棰佺窗鐠?func (r *SessionRepoImpl) Create(ctx context.Context, session *domain.Session) error {
	sessionDO := persistence.ToSessionDO(session)
	if err := r.dao.CreateSession(ctx, sessionDO); err != nil {
		return fmt.Errorf("mysql create session: %w", err)
	}
	return r.cache.SaveSession(ctx, session)
}

// Clear 濞撳懐鈹栨导姘崇樈
func (r *SessionRepoImpl) Clear(ctx context.Context, sessionID string) error {
	_ = r.cache.DeleteSession(ctx, sessionID)
	_ = r.dao.DeleteMessages(ctx, sessionID)
	return nil
}

// ListByUser 閸掑棝銆夐弻銉嚄閿涘牐铔?MySQL閿?func (r *SessionRepoImpl) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
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

// FindActiveByUser 閺屻儲澹橀悽銊﹀煕濞叉槒绌导姘崇樈
func (r *SessionRepoImpl) FindActiveByUser(ctx context.Context, userID int64) (*domain.Session, error) {
	// TODO: 閸欘垯绮?Redis agent:user:{uid}:active 韫囶偊鈧喐鐓￠幍?	sessions, _, err := r.ListByUser(ctx, userID, 1, 0)
	if err != nil || len(sessions) == 0 {
		return nil, err
	}
	if sessions[0].Status == domain.SessionActive {
		return &sessions[0], nil
	}
	return nil, nil
}


