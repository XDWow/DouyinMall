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

// SemanticCacheImpl 浜岀骇璇箟缂撳瓨锛歊edis hash + Milvus 灏忛泦鍚?
// 娴佺▼锛歟mbedding 鈫?Milvus 鏌ヨ繎浼煎悜閲?鈫?鍛戒腑涓斿垎鏁?> 闃堝€?鈫?Redis 鍙栧洖澶?
type SemanticCacheImpl struct {
	rdb       redis.Cmdable
	milvus    MilvusClient
	logger    logger.LoggerV1
	threshold float32       // 鍚戦噺鐩镐技搴﹂槇鍊硷紝榛樿 0.95
	ttl       time.Duration // Redis 缂撳瓨杩囨湡鏃堕棿
}

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

// Lookup 璇箟缂撳瓨鏌ユ壘
// 1. vector 鈫?Milvus agent_semantic_cache 闆嗗悎鏌?Top-1
// 2. 鍒嗘暟 > threshold 鈫?鐢ㄥ懡涓殑 ID 鍘?Redis 鍙栫紦瀛樺洖澶?
func (c *SemanticCacheImpl) Lookup(ctx context.Context, vector []float32) (string, bool) {
	hits, err := c.milvus.Search(ctx, CollectionCache, vector, 1)
	if err != nil || len(hits) == 0 {
		return "", false
	}

	hit := hits[0]
	if hit.Score < c.threshold {
		return "", false
	}

	// 浠?Redis 鍙栫紦瀛樺洖澶?
	reply, err := c.rdb.Get(ctx, c.cacheKey(hit.ID)).Result()
	if err != nil {
		// Milvus 鏈変絾 Redis 杩囨湡浜嗭紝涓嶇畻鍛戒腑
		return "", false
	}

	c.logger.Debug("璇箟缂撳瓨鍛戒腑",
		logger.String("vector_id", hit.ID),
		logger.Float64("score", float64(hit.Score)))
	return reply, true
}

// Store 瀛樺叆璇箟缂撳瓨
// 1. vector 鈫?Milvus agent_semantic_cache 闆嗗悎鍐欏叆
// 2. ID 鈫?Redis 瀛樺洖澶嶆枃鏈?
func (c *SemanticCacheImpl) Store(ctx context.Context, vector []float32, reply string) {
	id := vectorHash(vector)

	// 鍐?Milvus
	if err := c.milvus.Insert(ctx, CollectionCache, id, vector, nil); err != nil {
		c.logger.Warn("璇箟缂撳瓨鍐?Milvus 澶辫触", logger.Error(err))
		return
	}

	// 鍐?Redis
	if err := c.rdb.Set(ctx, c.cacheKey(id), reply, c.ttl).Err(); err != nil {
		c.logger.Warn("璇箟缂撳瓨鍐?Redis 澶辫触", logger.Error(err))
	}
}
