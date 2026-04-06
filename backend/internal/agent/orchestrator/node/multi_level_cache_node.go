package node

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// MultiLevelCacheInput 描述多级缓存节点需要的查询参数和策略。
type MultiLevelCacheInput struct {
	TenantID     string
	UserID       int64
	RawQuery     string
	SessionID    string
	TraceID      string
	CheckpointID string
	Policy       CachePolicyResult
}

// MultiLevelCacheResult 把缓存命中结果整理成统一输出，供主图决定是否短路。
type MultiLevelCacheResult struct {
	CacheHit    bool
	HitLevel    string
	Response    *domain.ChatResult
	FinalAnswer string
	Intent      domain.Intent
	Route       graphstate.WorkflowRoute
}

// MultiLevelCacheNode 依次尝试规范化精确缓存和语义缓存。
// 这样命中快路径时不需要提前进入完整意图识别和业务流程。
type MultiLevelCacheNode struct {
	ExactCache          cache.ExactCache
	SemanticCache       cache.SemanticCache
	Embedder            embedding.Embedder
	SemanticMinScore    float64
	SemanticLookupLimit int
}

func NewMultiLevelCacheNode(
	exactCache cache.ExactCache,
	semanticCache cache.SemanticCache,
	embedder embedding.Embedder,
	semanticMinScore float64,
	semanticLookupLimit int,
) *MultiLevelCacheNode {
	return &MultiLevelCacheNode{
		ExactCache:          exactCache,
		SemanticCache:       semanticCache,
		Embedder:            embedder,
		SemanticMinScore:    semanticMinScore,
		SemanticLookupLimit: semanticLookupLimit,
	}
}

func (n *MultiLevelCacheNode) Invoke(ctx context.Context, input MultiLevelCacheInput) (*MultiLevelCacheResult, error) {
	if input.Policy.AllowExact && n.ExactCache != nil {
		item, err := n.ExactCache.Lookup(ctx, input.TenantID, input.UserID, input.RawQuery)
		if err == nil && item != nil {
			return newCacheResult(input, item.Response, support.CacheHitExact), nil
		}
	}

	if !input.Policy.AllowSemantic || n.SemanticCache == nil || n.Embedder == nil {
		return &MultiLevelCacheResult{}, nil
	}

	vector, err := n.embedQuery(ctx, input.RawQuery)
	if err != nil || len(vector) == 0 {
		return &MultiLevelCacheResult{}, nil
	}

	item, err := n.SemanticCache.Lookup(ctx, cache.SemanticCacheLookup{
		TenantID:     input.TenantID,
		UserID:       input.UserID,
		IntentBucket: input.Policy.IntentBucket,
		Scope:        input.Policy.Scope,
		Vector:       vector,
		Threshold:    n.SemanticMinScore,
		Limit:        n.SemanticLookupLimit,
	})
	if err != nil || item == nil {
		return &MultiLevelCacheResult{}, nil
	}

	return newCacheResult(input, item.Response, support.CacheHitSemantic), nil
}

func (n *MultiLevelCacheNode) embedQuery(ctx context.Context, query string) ([]float64, error) {
	vectors, err := n.Embedder.EmbedStrings(ctx, []string{query})
	if err != nil || len(vectors) == 0 {
		return nil, err
	}
	return vectors[0], nil
}

func newCacheResult(input MultiLevelCacheInput, resp domain.ChatResult, hitLevel string) *MultiLevelCacheResult {
	resp.Trace.TraceID = input.TraceID
	resp.Trace.CheckpointID = input.CheckpointID
	resp.Trace.CacheHit = true
	resp.SessionID = input.SessionID

	return &MultiLevelCacheResult{
		CacheHit:    true,
		HitLevel:    hitLevel,
		Response:    &resp,
		FinalAnswer: resp.Reply,
		Intent:      resp.Intent,
		Route:       support.RouteFromIntent(resp.Intent),
	}
}
