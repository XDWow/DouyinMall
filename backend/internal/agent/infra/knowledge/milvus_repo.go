package knowledge

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// MilvusClient 抽象 Milvus 操作，隔离第三方 SDK
// ioc 中初始化时注入 milvus-sdk-go v2 的实际客户端
type MilvusClient interface {
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]VectorHit, error)
	Insert(ctx context.Context, collection string, id string, vector []float32, payload map[string]string) error
}

type VectorHit struct {
	ID      string
	Score   float32
	Payload map[string]string // title, snippet, category 等
}

const (
	CollectionKnowledge = "agent_knowledge"
	CollectionCache     = "agent_semantic_cache"
)

// MilvusKnowledgeRepo 基于 Milvus 实现 domain.KnowledgeRepo
type MilvusKnowledgeRepo struct {
	client MilvusClient
	logger logger.LoggerV1
}

func NewMilvusKnowledgeRepo(client MilvusClient, l logger.LoggerV1) domain.KnowledgeRepo {
	return &MilvusKnowledgeRepo{client: client, logger: l}
}

// VectorSearch 向量召回 Top-K 知识片段
func (r *MilvusKnowledgeRepo) VectorSearch(ctx context.Context, vector []float32, topK int) ([]domain.KnowledgeRef, error) {
	hits, err := r.client.Search(ctx, CollectionKnowledge, vector, topK)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	refs := make([]domain.KnowledgeRef, 0, len(hits))
	for _, hit := range hits {
		refs = append(refs, domain.KnowledgeRef{
			ID:        hit.ID,
			Title:     hit.Payload["title"],
			Snippet:   hit.Payload["snippet"],
			Category:  hit.Payload["category"],
			Relevance: hit.Score,
		})
	}
	return refs, nil
}
