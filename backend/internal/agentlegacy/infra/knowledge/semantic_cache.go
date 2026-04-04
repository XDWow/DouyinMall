//go:build legacy_agent

package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// SemanticCacheImpl 娴滃瞼楠囩拠顓濈疅缂傛挸鐡ㄩ敍姝奺dis hash + Milvus 鐏忓繘娉﹂崥?
// 濞翠胶鈻奸敍姝焟bedding 閳?Milvus 閺屻儴绻庢导鐓庢倻闁?閳?閸涙垝鑵戞稉鏂垮瀻閺?> 闂冨牆鈧?閳?Redis 閸欐牕娲栨径?
type SemanticCacheImpl struct {
	rdb       redis.Cmdable
	milvus    MilvusClient
	logger    logger.LoggerV1
	threshold float32       // 閸氭垿鍣洪惄闀愭妧鎼达箓妲囬崐纭风礉姒涙顓?0.95
	ttl       time.Duration // Redis 缂傛挸鐡ㄦ潻鍥ㄦ埂閺冨爼妫?}

func NewSemanticCache(
	rdb redis.Cmdable,
	milvus MilvusClient,
	l logger.LoggerV1,
) domain.SemanticCache {
	return &SemanticCacheImpl{
		rdb:       rdb,
		milvus:    milvus,
		logger:    l,
		threshold: 0.95,
		ttl:       1 * time.Hour,
	}
}

func (c *SemanticCacheImpl) cacheKey(vectorID string) string {
	return fmt.Sprintf("agent:cache:%s", vectorID)
}

func vectorHash(vector []float32) string {
	data, _ := json.Marshal(vector)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

// Lookup 鐠囶厺绠熺紓鎾崇摠閺屻儲澹?// 1. vector 閳?Milvus agent_semantic_cache 闂嗗棗鎮庨弻?Top-1
// 2. 閸掑棙鏆?> threshold 閳?閻劌鎳℃稉顓犳畱 ID 閸?Redis 閸欐牜绱︾€涙ê娲栨径?
func (c *SemanticCacheImpl) Lookup(ctx context.Context, vector []float32) (string, bool) {
	hits, err := c.milvus.Search(ctx, CollectionCache, vector, 1)
	if err != nil || len(hits) == 0 {
		return "", false
	}

	hit := hits[0]
	if hit.Score < c.threshold {
		return "", false
	}

	// 娴?Redis 閸欐牜绱︾€涙ê娲栨径?
	reply, err := c.rdb.Get(ctx, c.cacheKey(hit.ID)).Result()
	if err != nil {
		// Milvus 閺堝绲?Redis 鏉╁洦婀℃禍鍡礉娑撳秶鐣婚崨鎴掕厬
		return "", false
	}

	c.logger.Debug("鐠囶厺绠熺紓鎾崇摠閸涙垝鑵?,
		logger.String("vector_id", hit.ID),
		logger.Float64("score", float64(hit.Score)))
	return reply, true
}

// Store 鐎涙ê鍙嗙拠顓濈疅缂傛挸鐡?// 1. vector 閳?Milvus agent_semantic_cache 闂嗗棗鎮庨崘娆忓弳
// 2. ID 閳?Redis 鐎涙ê娲栨径宥嗘瀮閺?
func (c *SemanticCacheImpl) Store(ctx context.Context, vector []float32, reply string) {
	id := vectorHash(vector)

	// 閸?Milvus
	if err := c.milvus.Insert(ctx, CollectionCache, id, vector, nil); err != nil {
		c.logger.Warn("鐠囶厺绠熺紓鎾崇摠閸?Milvus 婢惰精瑙?, logger.Error(err))
		return
	}

	// 閸?Redis
	if err := c.rdb.Set(ctx, c.cacheKey(id), reply, c.ttl).Err(); err != nil {
		c.logger.Warn("鐠囶厺绠熺紓鎾崇摠閸?Redis 婢惰精瑙?, logger.Error(err))
	}
}


