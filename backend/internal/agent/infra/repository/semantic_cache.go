package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	sdkclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	exactCacheKeyPrefix            = "agent:exact:" // L1 精确缓存前缀
	cacheKeyPrefix                 = "agent:cache:" // L2 语义缓存前缀
	ragCacheKeyPrefix              = "agent:rag:"   // L3 RAG 缓存前缀
	cacheTTL                       = 1 * time.Hour
	semanticCacheThreshold float32 = 0.95
)

// 二级语义缓存：Milvus 向量检索 + Redis 回复存取
// Milvus 负责"找到语义最相似的历史问题"，Redis 负责"存/取对应的回复文本"
// 自身管理 key 规则（agent:cache:{vectorID}）和 TTL，通过 AgentCache 完成读写
type SemanticCacheImpl struct {
	redis  agentcache.AgentCache
	milvus sdkclient.Client
	logger logger.LoggerV1
}

func NewSemanticCache(
	redis agentcache.AgentCache,
	milvusClient sdkclient.Client,
	l logger.LoggerV1,
) domain.SemanticCache {
	return &SemanticCacheImpl{
		redis:  redis,
		milvus: milvusClient,
		logger: l,
	}
}

func vectorHash(vector []float32) string {
	data, _ := json.Marshal(vector)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

func queryHash(query string) string {
	h := sha256.Sum256([]byte(query))
	return fmt.Sprintf("%x", h[:8])
}

// L1: ExactLookup 精确缓存查找（Redis String，key = "exact:hash(query)"）
func (c *SemanticCacheImpl) ExactLookup(ctx context.Context, query string) (string, bool, error) {
	key := exactCacheKeyPrefix + queryHash(query)
	reply, err := c.redis.Get(ctx, key)
	if err != nil {
		return "", false, nil // Redis miss 不算错误
	}
	c.logger.Debug("精确缓存命中", logger.String("query_hash", queryHash(query)))
	return reply, true, nil
}

// L1: ExactStore 精确缓存存储
func (c *SemanticCacheImpl) ExactStore(ctx context.Context, query, reply string) error {
	key := exactCacheKeyPrefix + queryHash(query)
	if err := c.redis.Set(ctx, key, reply, cacheTTL); err != nil {
		c.logger.Warn("精确缓存写入失败", logger.Error(err))
		return fmt.Errorf("redis set exact cache: %w", err)
	}
	return nil
}

// L2: Lookup 语义缓存查找
// 1. vector → Milvus agent_semantic_cache 集合查 Top-1
// 2. score ≥ 0.95 → AgentRedis 取回复文本
func (c *SemanticCacheImpl) Lookup(ctx context.Context, vector []float32) (string, bool, error) {
	if c.milvus == nil {
		return "", false, nil
	}
	sp, err := entity.NewIndexFlatSearchParam()
	if err != nil {
		return "", false, fmt.Errorf("milvus search param: %w", err)
	}
	results, err := c.milvus.Search(ctx, domain.CollectionCache, nil, "", []string{},
		[]entity.Vector{entity.FloatVector(vector)}, "vector", entity.COSINE, 1, sp)
	if err != nil {
		return "", false, fmt.Errorf("milvus search cache: %w", err)
	}
	if len(results) == 0 || results[0].ResultCount == 0 || results[0].Scores[0] < semanticCacheThreshold {
		return "", false, nil
	}
	hitIDCol, ok := results[0].IDs.(*entity.ColumnVarChar)
	if !ok {
		return "", false, nil
	}
	hitID, hitScore := hitIDCol.Data()[0], results[0].Scores[0]

	reply, err := c.redis.Get(ctx, cacheKeyPrefix+hitID)
	if err != nil {
		// Milvus 有但 Redis 过期了，不算命中
		return "", false, nil
	}

	c.logger.Debug("语义缓存命中",
		logger.String("vector_id", hitID),
		logger.Float64("score", float64(hitScore)))
	return reply, true, nil
}

// L2: Store 存入语义缓存，Milvus 存向量 + ID，Redis 存答案内容
// 因为定位不同，Milvus 本质是 Vector Database，核心功能只有一个：高效做向量相似度搜索（ANN Search）
func (c *SemanticCacheImpl) Store(ctx context.Context, vector []float32, reply string) error {
	if c.milvus == nil {
		return nil
	}
	id := vectorHash(vector)

	idCol := entity.NewColumnVarChar("id", []string{id})
	vecCol := entity.NewColumnFloatVector("vector", len(vector), [][]float32{vector})
	if _, err := c.milvus.Insert(ctx, domain.CollectionCache, "", idCol, vecCol); err != nil {
		c.logger.Warn("语义缓存写 Milvus 失败", logger.Error(err))
		return fmt.Errorf("milvus insert cache: %w", err)
	}

	if err := c.redis.Set(ctx, cacheKeyPrefix+id, reply, cacheTTL); err != nil {
		c.logger.Warn("语义缓存写 Redis 失败", logger.Error(err))
		return fmt.Errorf("redis set cache reply: %w", err)
	}
	return nil
}

// L3: RAGLookup RAG 缓存查找（Redis Hash，存储检索结果）
func (c *SemanticCacheImpl) RAGLookup(ctx context.Context, vector []float32) ([]domain.KnowledgeRef, bool, error) {
	key := ragCacheKeyPrefix + vectorHash(vector)
	val, err := c.redis.Get(ctx, key)
	if err != nil {
		return nil, false, nil // Redis miss 不算错误
	}

	var knowledge []domain.KnowledgeRef
	if err := json.Unmarshal([]byte(val), &knowledge); err != nil {
		c.logger.Warn("RAG缓存反序列化失败", logger.Error(err))
		return nil, false, nil
	}

	c.logger.Debug("RAG缓存命中", logger.String("vector_hash", vectorHash(vector)))
	return knowledge, true, nil
}

// L3: RAGStore RAG 缓存存储
func (c *SemanticCacheImpl) RAGStore(ctx context.Context, vector []float32, knowledge []domain.KnowledgeRef) error {
	key := ragCacheKeyPrefix + vectorHash(vector)
	data, err := json.Marshal(knowledge)
	if err != nil {
		c.logger.Warn("RAG缓存序列化失败", logger.Error(err))
		return fmt.Errorf("json marshal knowledge: %w", err)
	}

	if err := c.redis.Set(ctx, key, string(data), cacheTTL); err != nil {
		c.logger.Warn("RAG缓存写入失败", logger.Error(err))
		return fmt.Errorf("redis set rag cache: %w", err)
	}
	return nil
}
