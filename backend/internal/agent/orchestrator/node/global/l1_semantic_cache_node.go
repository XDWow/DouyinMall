package global

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

const cacheHitSemantic = "semantic"

type L1SemanticCacheInput struct {
	TenantID     string
	UserID       int64
	Query        string
	SessionID    string
	TraceID      string
	CheckpointID string
	IntentBucket string
	Scope        cache.CacheScope
	AllowRead    bool
}

type L1SemanticCacheResult struct {
	CacheHit    bool
	HitLevel    string
	Response    *domain.ChatResult
	FinalAnswer string
	Intent      domain.Intent
	Route       graphstate.WorkflowRoute
}

type L1SemanticCacheNode struct {
	SemanticCache cache.SemanticCache
	Embedder      embedding.Embedder
	Threshold     float64
	TopK          int
}

func NewL1SemanticCacheNode(
	semanticCache cache.SemanticCache,
	embedder embedding.Embedder,
	threshold float64,
	topK int,
) *L1SemanticCacheNode {
	return &L1SemanticCacheNode{
		SemanticCache: semanticCache,
		Embedder:      embedder,
		Threshold:     threshold,
		TopK:          topK,
	}
}

func (n *L1SemanticCacheNode) Invoke(ctx context.Context, input L1SemanticCacheInput) (*L1SemanticCacheResult, error) {
	if !input.AllowRead || n == nil || n.SemanticCache == nil || n.Embedder == nil {
		return &L1SemanticCacheResult{}, nil
	}

	query := strings.TrimSpace(input.Query)
	if query == "" {
		return &L1SemanticCacheResult{}, nil
	}

	vectors, err := n.Embedder.EmbedStrings(ctx, []string{query})
	if err != nil || len(vectors) == 0 {
		return nil, err
	}

	item, err := n.SemanticCache.Lookup(ctx, cache.SemanticCacheLookup{
		TenantID:     input.TenantID,
		UserID:       input.UserID,
		IntentBucket: input.IntentBucket,
		Scope:        input.Scope,
		Vector:       vectors[0],
		Threshold:    n.Threshold,
		Limit:        n.TopK,
	})
	if err != nil || item == nil {
		return &L1SemanticCacheResult{}, err
	}

	return newSemanticCacheResult(input, item.Response), nil
}

func newSemanticCacheResult(input L1SemanticCacheInput, resp domain.ChatResult) *L1SemanticCacheResult {
	resp.Trace.TraceID = input.TraceID
	resp.Trace.CheckpointID = input.CheckpointID
	resp.Trace.CacheHit = true
	resp.SessionID = input.SessionID

	return &L1SemanticCacheResult{
		CacheHit:    true,
		HitLevel:    cacheHitSemantic,
		Response:    &resp,
		FinalAnswer: resp.Reply,
		Intent:      resp.Intent,
		Route:       routeFromIntent(resp.Intent),
	}
}
