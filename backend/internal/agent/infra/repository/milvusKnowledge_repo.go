package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentcache "github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agentdb "github.com/XDWow/DouyinMall/backend/internal/agent/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	sdkclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"gorm.io/gorm"
)

const knowledgeKeyPrefix = "agent:knowledge:" // Redis key 前缀

// Search 流程：
//
//	VectorStore.Search(collection, vector) → []{id, score}
//	Redis.MGet(ids)                        → 命中直接用（无 TTL，知识库稳定）
//	MySQL 补查 miss                      → 写回 Redis（无 TTL）
type MilvusKnowledgeRepo struct {
	milvusClient sdkclient.Client
	redis        agentcache.AgentCache
	db           *gorm.DB
	logger       logger.LoggerV1
}

func NewMilvusKnowledgeRepo(milvusClient sdkclient.Client, redis agentcache.AgentCache, db *gorm.DB, l logger.LoggerV1) domain.VectorRepo {
	return &MilvusKnowledgeRepo{milvusClient: milvusClient, redis: redis, db: db, logger: l}
}

func (r *MilvusKnowledgeRepo) Search(ctx context.Context, collection string, vector []float32, topK int) ([]domain.KnowledgeRef, error) {
	if r.milvusClient == nil {
		return nil, nil
	}

	// Step 1: 向量检索，返回 id + score
	sp, err := entity.NewIndexFlatSearchParam()
	if err != nil {
		return nil, fmt.Errorf("milvus search param: %w", err)
	}
	results, err := r.milvusClient.Search(ctx, collection, nil, "", []string{},
		[]entity.Vector{entity.FloatVector(vector)}, "vector", entity.COSINE, topK, sp)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	if len(results) == 0 || results[0].ResultCount == 0 {
		return nil, nil
	}
	sr := results[0]
	idCol, ok := sr.IDs.(*entity.ColumnVarChar)
	if !ok {
		return nil, fmt.Errorf("unexpected milvus ID type: %T", sr.IDs)
	}
	ids, scores := idCol.Data(), sr.Scores

	refs := make([]domain.KnowledgeRef, sr.ResultCount)
	keys := make([]string, sr.ResultCount)
	for i := 0; i < sr.ResultCount; i++ {
		keys[i] = knowledgeKeyPrefix + ids[i]
		refs[i] = domain.KnowledgeRef{
			ID:        ids[i],
			Relevance: scores[i],
		}
	}
	// 去 redis/mysql 拿 ref 其他信息
	// Step 2: Redis 批量查（无 TTL 的知识缓存层）
	cachedVals, err := r.redis.MGet(ctx, keys...)
	if err != nil {
		r.logger.Warn("知识缓存 MGet 失败，降级查 MySQL", logger.Error(err))
		cachedVals = make([]string, len(keys))
	}

	// Step 3: 命中就反序列化到 refs[i]，没命中收集 id 查数据库
	idToIndex := make(map[string]int)
	missIDs := make([]string, 0)
	for i, val := range cachedVals {
		if val != "" {
			if json.Unmarshal([]byte(val), &refs[i]) == nil {
				continue
			}
		}
		id := refs[i].ID
		missIDs = append(missIDs, id)
		idToIndex[id] = i
	}

	// Step 4: MySQL 补查 miss，写回 Redis
	if len(missIDs) > 0 {
		var items []agentdb.KnowledgeItem
		if err := r.db.WithContext(ctx).
			Where("vector_id IN ? AND status = 1", missIDs).
			Find(&items).Error; err != nil {
			r.logger.Warn("知识库 MySQL 回查失败", logger.Error(err))
		}
		for _, item := range items {
			idx := idToIndex[item.VectorID]
			ref := domain.KnowledgeRef{
				ID:        item.VectorID,
				Title:     item.Title,
				Content:   item.Content,
				Category:  item.Category,
				Relevance: refs[idx].Relevance,
			}
			refs[idx] = ref
			if b, e := json.Marshal(ref); e == nil {
				_ = r.redis.Set(ctx, knowledgeKeyPrefix+item.VectorID, string(b), 0) // 无 ttl
			}
		}
	}

	return refs, nil
}

// 向量入库
func (r *MilvusKnowledgeRepo) Insert(ctx context.Context, collection string, id string, vector []float32) error {
	if r.milvusClient == nil {
		return nil
	}
	idCol := entity.NewColumnVarChar("id", []string{id})
	vecCol := entity.NewColumnFloatVector("vector", len(vector), [][]float32{vector})
	_, err := r.milvusClient.Insert(ctx, collection, "", idCol, vecCol)
	return err
}
