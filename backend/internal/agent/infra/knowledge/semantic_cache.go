package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// SemanticCacheImpl 二级语义缓存：Redis hash + Milvus 小集合
// 流程：embedding → Milvus 查近似向量 → 命中且分数 > 阈值 → Redis 取回复
type SemanticCacheImpl struct {
	rdb       redis.Cmdable
	milvus    MilvusClient
	logger    logger.LoggerV1
	threshold float32       // 向量相似度阈值，默认 0.95
	ttl       time.Duration // Redis 缓存过期时间
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

// Lookup 语义缓存查找
// 1. vector → Milvus agent_semantic_cache 集合查 Top-1
// 2. 分数 > threshold → 用命中的 ID 去 Redis 取缓存回复
func (c *SemanticCacheImpl) Lookup(ctx context.Context, vector []float32) (string, bool) {
	hits, err := c.milvus.Search(ctx, CollectionCache, vector, 1)
	if err != nil || len(hits) == 0 {
		return "", false
	}

	hit := hits[0]
	if hit.Score < c.threshold {
		return "", false
	}

	// 从 Redis 取缓存回复
	reply, err := c.rdb.Get(ctx, c.cacheKey(hit.ID)).Result()
	if err != nil {
		// Milvus 有但 Redis 过期了，不算命中
		return "", false
	}

	c.logger.Debug("语义缓存命中",
		logger.String("vector_id", hit.ID),
		logger.Float64("score", float64(hit.Score)))
	return reply, true
}

// Store 存入语义缓存
// 1. vector → Milvus agent_semantic_cache 集合写入
// 2. ID → Redis 存回复文本
func (c *SemanticCacheImpl) Store(ctx context.Context, vector []float32, reply string) {
	id := vectorHash(vector)

	// 写 Milvus
	if err := c.milvus.Insert(ctx, CollectionCache, id, vector, nil); err != nil {
		c.logger.Warn("语义缓存写 Milvus 失败", logger.Error(err))
		return
	}

	// 写 Redis
	if err := c.rdb.Set(ctx, c.cacheKey(id), reply, c.ttl).Err(); err != nil {
		c.logger.Warn("语义缓存写 Redis 失败", logger.Error(err))
	}
}
