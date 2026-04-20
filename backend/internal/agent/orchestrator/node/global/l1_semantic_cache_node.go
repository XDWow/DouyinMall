package global

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

const cacheHitSemantic = "semantic"

type L1SemanticCacheNode struct {
	SemanticCache   cache.SemanticCache
	Embedder        embedding.Embedder
	Threshold       float64
	TopK            int
	DefaultTenantID string
}

func NewL1SemanticCacheNode(
	semanticCache cache.SemanticCache,
	embedder embedding.Embedder,
	threshold float64,
	topK int,
	defaultTenantID string,
) *L1SemanticCacheNode {
	return &L1SemanticCacheNode{
		SemanticCache:   semanticCache,
		Embedder:        embedder,
		Threshold:       threshold,
		TopK:            topK,
		DefaultTenantID: defaultTenantID,
	}
}

type L1SemanticCacheInput struct {
	TenantID  string
	UserID    int64
	SessionID string
	Intent    domain.Intent
	Query     string
}

type L1SemanticCacheResult struct {
	Hit      bool
	Response *domain.ChatResult
}

func (n *L1SemanticCacheNode) Invoke(ctx context.Context, in L1SemanticCacheInput) (L1SemanticCacheResult, error) {
	if n == nil || n.SemanticCache == nil || n.Embedder == nil {
		return L1SemanticCacheResult{}, nil
	}
	if !semanticCacheableIntent(in.Intent) {
		return L1SemanticCacheResult{}, nil
	}
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return L1SemanticCacheResult{}, nil
	}

	vectors, err := n.Embedder.EmbedStrings(ctx, []string{query})
	if err != nil || len(vectors) == 0 {
		return L1SemanticCacheResult{}, err
	}

	item, err := n.SemanticCache.Lookup(ctx, cache.SemanticCacheLookup{
		TenantID:     firstNonEmpty(in.TenantID, n.DefaultTenantID),
		UserID:       in.UserID,
		IntentBucket: semanticIntentBucket(in.Intent),
		Scope:        semanticScopeForIntent(in.Intent),
		Vector:       vectors[0],
		Threshold:    n.Threshold,
		Limit:        n.TopK,
	})
	if err != nil || item == nil {
		return L1SemanticCacheResult{}, err
	}
	resp := item.Response
	if !semanticCacheableIntent(resp.Intent) {
		return L1SemanticCacheResult{}, nil
	}
	resp.SessionID = in.SessionID
	resp.Trace.CacheHit = true
	return L1SemanticCacheResult{
		Hit:      true,
		Response: &resp,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
