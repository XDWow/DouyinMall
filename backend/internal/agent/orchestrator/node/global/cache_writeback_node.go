package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type CacheWritebackService struct {
	ExactCache       cache.ExactCache
	SemanticCache    cache.SemanticCache
	Embedder         embedding.Embedder
	ExactCacheTTL    time.Duration
	SemanticCacheTTL time.Duration
	Logger           logger.LoggerV1
}

func NewCacheWritebackService(
	exactCache cache.ExactCache,
	semanticCache cache.SemanticCache,
	embedder embedding.Embedder,
	exactCacheTTL time.Duration,
	semanticCacheTTL time.Duration,
	log logger.LoggerV1,
) *CacheWritebackService {
	return &CacheWritebackService{
		ExactCache:       exactCache,
		SemanticCache:    semanticCache,
		Embedder:         embedder,
		ExactCacheTTL:    exactCacheTTL,
		SemanticCacheTTL: semanticCacheTTL,
		Logger:           log,
	}
}

func (n *CacheWritebackService) Write(ctx context.Context, state *domain.State) error {
	if state == nil {
		return fmt.Errorf("state is required")
	}

	resp := state.EnsureResponse()
	resp.Trace.TraceID = state.TraceID
	resp.Trace.CheckpointID = state.Checkpoint
	resp.Trace.CacheHit = state.Session.CacheHitLevel != ""
	resp.Trace.RewrittenQuery = state.Rewrite.Query

	if !n.shouldWriteCache(state, resp) {
		return nil
	}

	cacheIntent := n.cacheIntent(state, resp)
	if n.ExactCache != nil && n.ExactCacheTTL > 0 && n.shouldWriteExactCache(state, cacheIntent) {
		_ = n.ExactCache.Store(ctx, &cache.ExactCacheItem{
			TenantID: state.Session.TenantID,
			UserID:   state.Input.UserID,
			Query:    state.Session.RawQuery,
			Response: *resp,
		}, n.ExactCacheTTL)
	}

	policy := ResolveSemanticCachePolicy(state.Session.Route, state.Session.RawQuery)
	if n.SemanticCache != nil && n.Embedder != nil && n.SemanticCacheTTL > 0 && n.shouldWriteSemanticCache(state, policy) {
		query := strings.TrimSpace(support.FirstNonEmpty(state.Rewrite.Query, state.Session.RawQuery))
		if query != "" {
			if vector, err := n.embedQuery(ctx, query); err == nil && len(vector) > 0 {
				userID := int64(0)
				if policy.Scope == cache.CacheScopeTenantUser {
					userID = state.Input.UserID
				}
				_ = n.SemanticCache.Store(ctx, &cache.SemanticCacheItem{
					TenantID:     state.Session.TenantID,
					UserID:       userID,
					IntentBucket: policy.IntentBucket,
					Scope:        policy.Scope,
					Query:        query,
					Response:     *resp,
					Vector:       vector,
				}, n.SemanticCacheTTL)
			}
		}
	}

	return nil
}

func (n *CacheWritebackService) cacheIntent(state *domain.State, resp *domain.ChatResult) domain.Intent {
	if resp != nil && resp.Intent != domain.IntentUnknown {
		return resp.Intent
	}
	if state.Session.Intent != domain.IntentUnknown {
		return state.Session.Intent
	}
	return detectCacheIntent(state.Session.RawQuery)
}

func (n *CacheWritebackService) shouldWriteExactCache(state *domain.State, intent domain.Intent) bool {
	if state != nil && state.Answer.CacheableHint != nil && !*state.Answer.CacheableHint {
		return false
	}

	switch intent {
	case domain.IntentReturnPolicy:
		return true
	case domain.IntentProductInfo:
		msg := normalizeCacheQuery(state.Session.RawQuery)
		return msg != "" && isStableProductKnowledgeQuery(msg) && !hasDynamicCacheTool(state)
	default:
		return false
	}
}

func (n *CacheWritebackService) shouldWriteSemanticCache(state *domain.State, policy SemanticCachePolicy) bool {
	if !policy.AllowRead {
		return false
	}
	if state != nil && state.Answer.CacheableHint != nil && !*state.Answer.CacheableHint {
		return false
	}

	switch state.Session.Route {
	case domain.RouteProductInfo:
		return !hasDynamicCacheTool(state)
	case domain.RouteBaseQA:
		return len(state.Retrieval.Documents) > 0
	default:
		return true
	}
}

func (n *CacheWritebackService) shouldWriteCache(state *domain.State, resp *domain.ChatResult) bool {
	return resp != nil &&
		state.Session.ReadOnly &&
		state.Session.CacheHitLevel == "" &&
		resp.Confidence >= lowConfidenceThreshold &&
		!resp.NeedHandoff &&
		!state.Session.AwaitingUser &&
		!state.Session.AwaitingConfirm &&
		strings.TrimSpace(resp.Reply) != ""
}

func (n *CacheWritebackService) embedQuery(ctx context.Context, query string) ([]float64, error) {
	vectors, err := n.Embedder.EmbedStrings(ctx, []string{query})
	if err != nil || len(vectors) == 0 {
		return nil, err
	}
	return vectors[0], nil
}

func hasDynamicCacheTool(state *domain.State) bool {
	for _, plan := range state.Tool.Plans {
		if isDynamicCacheTool(plan.Name) {
			return true
		}
	}
	for _, name := range state.EnsureResponse().UsedToolNames {
		if isDynamicCacheTool(name) {
			return true
		}
	}
	for _, exec := range state.EnsureResponse().ToolExecutions {
		if isDynamicCacheTool(exec.Name) {
			return true
		}
	}
	return false
}

func isDynamicCacheTool(name string) bool {
	switch strings.TrimSpace(name) {
	case "query_order", "list_user_orders", "get_order", "get_inventory", "add_to_cart", "create_after_sale_request":
		return true
	default:
		return false
	}
}
