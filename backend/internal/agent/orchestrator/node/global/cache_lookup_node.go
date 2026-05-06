package global

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

const cacheHitSemantic = "semantic"
const cacheHitExact = "exact"

type CacheLookupNode struct {
	ExactCache      cache.ExactCache
	SemanticCache   cache.SemanticCache
	Embedder        embedding.Embedder
	Threshold       float64
	TopK            int
	DefaultTenantID string
}

func NewCacheLookupNode(
	exactCache cache.ExactCache,
	semanticCache cache.SemanticCache,
	embedder embedding.Embedder,
	threshold float64,
	topK int,
	defaultTenantID string,
) *CacheLookupNode {
	return &CacheLookupNode{
		ExactCache:      exactCache,
		SemanticCache:   semanticCache,
		Embedder:        embedder,
		Threshold:       threshold,
		TopK:            topK,
		DefaultTenantID: defaultTenantID,
	}
}

type CacheLookupInput struct {
	TenantID       string
	UserID         int64
	SessionID      string
	TraceID        string
	Intent         domain.Intent
	Query          string
	RewrittenQuery string
}

type CacheLookupResult struct {
	Hit      bool                 `json:"hit"`
	Source   string               `json:"source,omitempty"`
	Route    domain.WorkflowRoute `json:"route"`
	Response *domain.ChatResult   `json:"response,omitempty"`
}

func (n *CacheLookupNode) Invoke(ctx context.Context, in CacheLookupInput) (CacheLookupResult, error) {
	result := CacheLookupResult{Route: domain.WorkflowRouteFromIntent(in.Intent)}
	if n == nil {
		return result, nil
	}

	stateCacheable := domain.DefaultReadOnlyForIntent(in.Intent) &&
		(exactCacheableIntent(in.Intent) || semanticCacheableIntent(in.Intent))
	if st := domain.SharedGraphState(ctx); st != nil && st.Input != nil {
		stateCacheable = CanReadCache(st)
	}
	if !stateCacheable {
		return result, nil
	}

	query := strings.TrimSpace(in.Query)
	if query == "" {
		return result, nil
	}
	tenantID := firstNonEmpty(in.TenantID, n.DefaultTenantID, "default")

	if n.ExactCache != nil && exactCacheableIntent(in.Intent) {
		item, err := n.ExactCache.Lookup(ctx, tenantID, in.UserID, query)
		if err != nil {
			return result, err
		}
		if item != nil {
			resp := n.prepareHitResponse(item.Response, in)
			if exactCacheableIntent(resp.Intent) {
				result.Hit = true
				result.Source = cacheHitExact
				result.Response = &resp
				return result, nil
			}
		}
	}

	if n.SemanticCache == nil || n.Embedder == nil || !semanticCacheableIntent(in.Intent) {
		return result, nil
	}

	semanticQuery := strings.TrimSpace(support.FirstNonEmpty(in.RewrittenQuery, query))
	if semanticQuery == "" {
		return result, nil
	}
	vectors, err := n.Embedder.EmbedStrings(ctx, []string{semanticQuery})
	if err != nil || len(vectors) == 0 {
		return result, err
	}

	item, err := n.SemanticCache.Lookup(ctx, cache.SemanticCacheLookup{
		TenantID:     tenantID,
		UserID:       in.UserID,
		IntentBucket: semanticIntentBucket(in.Intent),
		Scope:        semanticScopeForIntent(in.Intent),
		Vector:       vectors[0],
		Threshold:    n.Threshold,
		Limit:        n.TopK,
	})
	if err != nil || item == nil {
		return result, err
	}
	resp := n.prepareHitResponse(item.Response, in)
	if !semanticCacheableIntent(resp.Intent) {
		return result, nil
	}
	return CacheLookupResult{
		Hit:      true,
		Source:   cacheHitSemantic,
		Route:    result.Route,
		Response: &resp,
	}, nil
}

func (n *CacheLookupNode) prepareHitResponse(resp domain.ChatResult, in CacheLookupInput) domain.ChatResult {
	resp.SessionID = in.SessionID
	resp.TraceID = in.TraceID
	resp.Trace.TraceID = in.TraceID
	resp.Trace.CacheHit = true
	resp.Trace.RewrittenQuery = strings.TrimSpace(in.RewrittenQuery)
	if resp.Intent == "" {
		resp.Intent = in.Intent
	}
	if resp.Status == "" {
		resp.Status = domain.ReplyStatusAnswered
	}
	if resp.Confidence <= 0 {
		resp.Confidence = 0.95
	}
	if resp.Trace.Steps == nil {
		resp.Trace.Steps = nil
	}
	if resp.ToolExecutions == nil {
		resp.ToolExecutions = nil
	}
	if resp.UsedToolNames == nil {
		resp.UsedToolNames = nil
	}
	resp.Interrupt = nil
	resp.Interrupted = false
	resp.Streamed = false
	return resp
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
