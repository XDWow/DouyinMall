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

// RedisSessionCache Redis 浼氳瘽鐑眰
// 瀹炵幇 domain.SessionRepo 鐨?Redis 閮ㄥ垎锛圠oad/Save/Create/Clear锛?// MySQL 鍐峰眰鐢?persistence 鍖呰礋璐ｏ紝閫氳繃缁勫悎瀹炵幇瀹屾暣鐨?SessionRepo
type RedisSessionCache struct {
	client redis.Cmdable
}

func NewRedisSessionCache(client redis.Cmdable) *RedisSessionCache {
	return &RedisSessionCache{client: client}
}

// SaveSession 淇濆瓨浼氳瘽鍏冧俊鎭埌 Redis Hash
func (c *RedisSessionCache) SaveSession(ctx context.Context, session *domain.Session) error {
	key := sessionKeyPrefix + session.ID
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	pipe := c.client.Pipeline()
	pipe.Set(ctx, key, data, sessionTTL)
	// 鏇存柊鐢ㄦ埛娲昏穬浼氳瘽绱㈠紩
	activeKey := fmt.Sprintf("%s%d%s", activeKeyPrefix, session.UserID, activeKeySuffix)
	pipe.Set(ctx, activeKey, session.ID, sessionTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// LoadSession 浠?Redis 鍔犺浇浼氳瘽
func (c *RedisSessionCache) LoadSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	key := sessionKeyPrefix + sessionID
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err // redis.Nil 琛ㄧず miss
	}
	var session domain.Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

// AppendMessage 杩藉姞娑堟伅鍒?Redis List锛堟粦鍔ㄧ獥鍙ｏ紝淇濈暀鏈€杩?N 鏉★級
func (c *RedisSessionCache) AppendMessage(ctx context.Context, sessionID string, msg domain.Message) error {
	key := sessionKeyPrefix + sessionID + msgsKeySuffix
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	pipe := c.client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.LTrim(ctx, key, -maxWindowMsgs, -1) // 鍙繚鐣欐渶杩?N 鏉?	pipe.Expire(ctx, key, sessionTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// LoadMessages 浠?Redis 鍔犺浇婊戝姩绐楀彛鍐呯殑娑堟伅
func (c *RedisSessionCache) LoadMessages(ctx context.Context, sessionID string) ([]domain.Message, error) {
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

// DeleteSession 鍒犻櫎浼氳瘽缂撳瓨
func (c *RedisSessionCache) DeleteSession(ctx context.Context, sessionID string) error {
	keys := []string{
		sessionKeyPrefix + sessionID,
		sessionKeyPrefix + sessionID + msgsKeySuffix,
	}
	return c.client.Del(ctx, keys...).Err()
}

// CheckRateLimit 娑堟伅闄愰锛? 鍒嗛挓鍐呮槸鍚﹁秴杩?limit 鏉?func (c *RedisSessionCache) CheckRateLimit(ctx context.Context, userID int64, limit int64) (bool, error) {
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
