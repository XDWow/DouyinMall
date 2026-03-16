package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentdb "github.com/XDWow/DouyinMall/backend/internal/agent/infra/db"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"gorm.io/gorm"
)

// key 规则统一在 repo 层管理，cache 只做纯 Redis 操作
const (
	sessionKeyPrefix = "agent:session:"
	msgsKeySuffix    = ":msgs"
	sessionTTL       = 24 * time.Hour
	// maxWindowMsgs 必须与 usecase.maxWindowSize 保持一致，
	// 保证内存窗口和缓存窗口始终对齐。
	maxWindowMsgs = 10
)

// appendMsgsScript 批量追加消息，一次原子完成 RPush + LTrim + Expire
// ARGV 布局：[msg1, msg2, ..., msgN, windowSize, ttlSeconds]
const appendMsgsScript = `
local n = #ARGV
redis.call('RPUSH', KEYS[1], unpack(ARGV, 1, n-2))
redis.call('LTRIM', KEYS[1], 0 - tonumber(ARGV[n-1]), -1)
redis.call('EXPIRE', KEYS[1], ARGV[n])
return 1
`

// sessionRepo 组合 Redis 热层 + Kafka 异步落库 + MySQL 冷层
// 写路径：Redis（实时）+ Kafka（异步持久化）
// 读路径：Redis -> MySQL 回源
// cache 只做纯 Redis 操作，key 构造和序列化在此层完成
type sessionRepo struct {
	cache    cache.AgentCache
	producer *mq.MessageProducer
	db       *gorm.DB
	logger   logger.LoggerV1
}

func NewSessionRepo(
	c cache.AgentCache,
	producer *mq.MessageProducer,
	db *gorm.DB,
	logger logger.LoggerV1,
) domain.SessionRepo {
	return &sessionRepo{cache: c, producer: producer, db: db, logger: logger}
}

// 加载会话元信息（优先 Redis，miss 回源 MySQL）
// 不含消息列表，适用于需要判断会话状态、获取 userID 等纯元数据场景
func (r *sessionRepo) LoadSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	key := sessionKeyPrefix + sessionID
	if raw, err := r.cache.Get(ctx, key); err == nil {
		var s domain.Session
		if json.Unmarshal([]byte(raw), &s) == nil {
			return &s, nil
		}
	}

	// MySQL
	var do agentdb.Session
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&do).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	s := toDomainSession(&do)

	// 回填 Redis（异步，不阻塞主链路）
	go func() {
		if b, e := json.Marshal(s); e == nil {
			_ = r.cache.Set(context.Background(), key, string(b), sessionTTL)
		}
	}()
	return s, nil
}

// 加载会话消息窗口（优先 Redis，miss 回源 MySQL）
// 只返回消息列表，不含会话元信息
func (r *sessionRepo) LoadMessages(ctx context.Context, sessionID string) ([]domain.Message, error) {
	key := sessionKeyPrefix + sessionID + msgsKeySuffix
	if raws, err := r.cache.LRange(ctx, key, 0, -1); err == nil && len(raws) > 0 {
		msgs := make([]domain.Message, 0, len(raws))
		for _, raw := range raws {
			var m domain.Message
			if json.Unmarshal([]byte(raw), &m) == nil {
				msgs = append(msgs, m)
			}
		}
		return msgs, nil
	}

	var dos []agentdb.Message
	r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(maxWindowMsgs).
		Find(&dos)

	// 翻转 slice，让结果从旧到新
	slices.Reverse(dos)
	msgs := make([]domain.Message, len(dos))
	for i, do := range dos {
		msgs[i] = toDomainMessage(&do)
	}

	// 异步回填 redis
	go func() {
		ctx := context.Background()
		vals := make([]string, 0, len(msgs))
		for _, m := range msgs {
			if b, e := json.Marshal(m); e == nil {
				vals = append(vals, string(b))
			}
		}
		if len(vals) == 0 {
			return
		}
		_ = r.cache.RPush(ctx, key, vals...)
		_ = r.cache.Expire(ctx, key, sessionTTL)
	}()
	return msgs, nil
}

// 追加本轮新消息：写 Redis 热层 + 投递 Kafka 异步落库
func (r *sessionRepo) AppendMessages(ctx context.Context, session *domain.Session, newMsgs []domain.Message) error {
	// 更新会话元信息到 Redis
	if b, err := json.Marshal(session); err == nil {
		_ = r.cache.Set(ctx, sessionKeyPrefix+session.ID, string(b), sessionTTL)
	}

	// 追加两条消息：Lua 脚本保证 RPush+LTrim+Expire 原子性
	msgsKey := sessionKeyPrefix + session.ID + msgsKeySuffix
	args := make([]any, 0, len(newMsgs)+2)
	for _, msg := range newMsgs {
		if b, err := json.Marshal(msg); err == nil {
			args = append(args, string(b))
		}
	}
	if len(args) > 0 {
		args = append(args, maxWindowMsgs, int64(sessionTTL.Seconds()))
		_, _ = r.cache.Eval(ctx, appendMsgsScript, []string{msgsKey}, args...)
	}

	// Kafka 异步落库
	evt := domain.ChatMessageEvent{SessionID: session.ID, Messages: newMsgs}
	if err := r.producer.ProduceMessages(ctx, evt); err != nil {
		r.logger.Error("Kafka 投递消息失败",
			logger.String("session", session.ID), logger.Error(err))
	}
	return nil
}

// 将会话元信息刷写到 MySQL（仅在会话终态时调用，运行时 Redis 是唯一来源）
func (r *sessionRepo) FlushSession(ctx context.Context, session *domain.Session) error {
	return r.db.WithContext(ctx).Model(&agentdb.Session{}).
		Where("session_id = ?", session.ID).
		Updates(map[string]any{
			"status":               uint8(session.Status),
			"low_confidence_turns": session.LowConfidenceTurns,
		}).Error
}

// 创建新会话（写 MySQL + Redis）
func (r *sessionRepo) Create(ctx context.Context, session *domain.Session) error {
	if err := r.db.WithContext(ctx).Create(toSessionDO(session)).Error; err != nil {
		return fmt.Errorf("mysql create session: %w", err)
	}
	if b, err := json.Marshal(session); err == nil {
		_ = r.cache.Set(ctx, sessionKeyPrefix+session.ID, string(b), sessionTTL)
	}
	return nil
}

// 清空会话（Redis 元信息 + 消息 key，以及 MySQL 消息表）
func (r *sessionRepo) Clear(ctx context.Context, sessionID string) error {
	_ = r.cache.Del(ctx,
		sessionKeyPrefix+sessionID,
		sessionKeyPrefix+sessionID+msgsKeySuffix,
	)
	_ = r.db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&agentdb.Message{}).Error
	return nil
}

// 分页查询，完整信息在 mysql
func (r *sessionRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	var dos []agentdb.Session
	var total int64
	query := r.db.WithContext(ctx).Where("user_id = ?", uint64(userID))
	if err := query.Model(&agentdb.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&dos).Error; err != nil {
		return nil, 0, err
	}
	sessions := make([]domain.Session, len(dos))
	for i, do := range dos {
		sessions[i] = *toDomainSession(&do)
	}
	return sessions, int(total), nil
}

// 转换函数
func toDomainSession(do *agentdb.Session) *domain.Session {
	return &domain.Session{
		ID:                 do.SessionID,
		UserID:             int64(do.UserID),
		Channel:            do.Channel,
		Status:             domain.SessionStatus(do.Status),
		LowConfidenceTurns: do.LowConfidenceTurns,
		CreatedAt:          do.CreatedAt,
		UpdatedAt:          do.UpdatedAt,
	}
}

func toSessionDO(s *domain.Session) *agentdb.Session {
	return &agentdb.Session{
		SessionID:          s.ID,
		UserID:             uint64(s.UserID),
		Channel:            s.Channel,
		Status:             uint8(s.Status),
		LowConfidenceTurns: s.LowConfidenceTurns,
	}
}

func toDomainMessage(do *agentdb.Message) domain.Message {
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
