//go:build legacy_agent

package knowledge

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// MilvusClient 閹跺€熻杽 Milvus 閹垮秳缍旈敍宀勬缁傝崵顑囨稉澶嬫煙 SDK
// ioc 娑擃厼鍨垫慨瀣閺冭埖鏁為崗?milvus-sdk-go v2 閻ㄥ嫬鐤勯梽鍛吂閹撮顏?type MilvusClient interface {
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]VectorHit, error)
	Insert(ctx context.Context, collection string, id string, vector []float32, payload map[string]string) error
}

type VectorHit struct {
	ID      string
	Score   float32
	Payload map[string]string // title, snippet, category 缁?}

const (
	CollectionKnowledge = "agent_knowledge"
	CollectionCache     = "agent_semantic_cache"
)

// MilvusKnowledgeRepo 閸╄桨绨?Milvus 鐎圭偟骞?domain.KnowledgeRepo
type MilvusKnowledgeRepo struct {
	client MilvusClient
	logger logger.LoggerV1
}

func NewMilvusKnowledgeRepo(client MilvusClient, l logger.LoggerV1) domain.KnowledgeRepo {
	return &MilvusKnowledgeRepo{client: client, logger: l}
}

// VectorSearch 閸氭垿鍣洪崣顒€娲?Top-K 閻儴鐦戦悧鍥唽
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


