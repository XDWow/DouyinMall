//go:build legacy_agent

package knowledge

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// MilvusClient 鎶借薄 Milvus 鎿嶄綔锛岄殧绂荤涓夋柟 SDK
// ioc 涓垵濮嬪寲鏃舵敞鍏?milvus-sdk-go v2 鐨勫疄闄呭鎴风
type MilvusClient interface {
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]VectorHit, error)
	Insert(ctx context.Context, collection string, id string, vector []float32, payload map[string]string) error
}

type VectorHit struct {
	ID      string
	Score   float32
	Payload map[string]string // title, snippet, category 绛?}

const (
	CollectionKnowledge = "agent_knowledge"
	CollectionCache     = "agent_semantic_cache"
)

// MilvusKnowledgeRepo 鍩轰簬 Milvus 瀹炵幇 domain.KnowledgeRepo
type MilvusKnowledgeRepo struct {
	client MilvusClient
	logger logger.LoggerV1
}

func NewMilvusKnowledgeRepo(client MilvusClient, l logger.LoggerV1) domain.KnowledgeRepo {
	return &MilvusKnowledgeRepo{client: client, logger: l}
}

// VectorSearch 鍚戦噺鍙洖 Top-K 鐭ヨ瘑鐗囨
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
