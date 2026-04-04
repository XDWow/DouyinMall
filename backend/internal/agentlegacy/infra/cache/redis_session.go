//go:build legacy_agent

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	"github.com/redis/go-redis/v9"
)

const (
	sessionKeyPrefix = "agent:session:"
	msgsKeySuffix    = ":msgs"
	activeKeyPrefix  = "agent:user:"
	activeKeySuffix  = ":active"
	rateKeyPrefix    = "agent:rate:"
	sessionTTL       = 24 * time.Hour
	rateTTL          = time.Minute
	maxWindowMsgs    = 20
)

// RedisSessionCache Redis 娴兼俺鐦介悜顓炵湴
// 鐎圭偟骞?domain.SessionRepo 閻?Redis 闁劌鍨庨敍鍦爋ad/Save/Create/Clear閿?// MySQL 閸愬嘲鐪伴悽?persistence 閸栧懓绀嬬拹锝忕礉闁俺绻冪紒鍕値鐎圭偟骞囩€瑰本鏆ｉ惃?SessionRepo
type RedisSessionCache struct {
	client redis.Cmdable
}

func NewRedisSessionCache(client redis.Cmdable) *RedisSessionCache {
	return &RedisSessionCache{client: client}
}

// SaveSession 娣囨繂鐡ㄦ导姘崇樈閸忓啩淇婇幁顖氬煂 Redis Hash
func (c *RedisSessionCache) SaveSession(ctx context.Context, session *domain.Session) error {
	key := sessionKeyPrefix + session.ID
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	pipe := c.client.Pipeline()
	pipe.Set(ctx, key, data, sessionTTL)
	// 閺囧瓨鏌婇悽銊﹀煕濞叉槒绌导姘崇樈缁便垹绱?	activeKey := fmt.Sprintf("%s%d%s", activeKeyPrefix, session.UserID, activeKeySuffix)
	pipe.Set(ctx, activeKey, session.ID, sessionTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// LoadSession 娴?Redis 閸旂姾娴囨导姘崇樈
func (c *RedisSessionCache) LoadSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	key := sessionKeyPrefix + sessionID
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err // redis.Nil 鐞涖劎銇?miss
	}
	var session domain.Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

// AppendMessage 鏉╄棄濮炲☉鍫熶紖閸?Redis List閿涘牊绮﹂崝銊х崶閸欙綇绱濇穱婵堟殌閺堚偓鏉?N 閺夆槄绱?func (c *RedisSessionCache) AppendMessage(ctx context.Context, sessionID string, msg domain.Message) error {
	key := sessionKeyPrefix + sessionID + msgsKeySuffix
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	pipe := c.client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.LTrim(ctx, key, -maxWindowMsgs, -1) // 閸欘亙绻氶悾娆愭付鏉?N 閺?	pipe.Expire(ctx, key, sessionTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// LoadMessages 娴?Redis 閸旂姾娴囧鎴濆З缁愭褰涢崘鍛畱濞戝牊浼?func (c *RedisSessionCache) LoadMessages(ctx context.Context, sessionID string) ([]domain.Message, error) {
	key := sessionKeyPrefix + sessionID + msgsKeySuffix
	results, err := c.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	msgs := make([]domain.Message, 0, len(results))
	for _, raw := range results {
		var msg domain.Message
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			continue
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// DeleteSession 閸掔娀娅庢导姘崇樈缂傛挸鐡?func (c *RedisSessionCache) DeleteSession(ctx context.Context, sessionID string) error {
	keys := []string{
		sessionKeyPrefix + sessionID,
		sessionKeyPrefix + sessionID + msgsKeySuffix,
	}
	return c.client.Del(ctx, keys...).Err()
}

// CheckRateLimit 濞戝牊浼呴梽鎰邦暥閿? 閸掑棝鎸撻崘鍛Ц閸氾箒绉存潻?limit 閺?func (c *RedisSessionCache) CheckRateLimit(ctx context.Context, userID int64, limit int64) (bool, error) {
	key := fmt.Sprintf("%s%d", rateKeyPrefix, userID)
	count, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		c.client.Expire(ctx, key, rateTTL)
	}
	return count > limit, nil
}


