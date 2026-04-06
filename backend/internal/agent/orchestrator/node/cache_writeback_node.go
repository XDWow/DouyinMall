package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// CacheWritebackNode 负责持久化会话，并把可复用的回答回写到多级缓存。
// 精确缓存面向完全相同的规范化问句，语义缓存只面向稳定知识问答。
type CacheWritebackNode struct {
	ExactCache       cache.ExactCache
	SemanticCache    cache.SemanticCache
	Embedder         embedding.Embedder
	ExactCacheTTL    time.Duration
	SemanticCacheTTL time.Duration
	PersistTurn      ConversationTurnPersister
	Logger           logger.LoggerV1
}

func NewCacheWritebackNode(
	exactCache cache.ExactCache,
	semanticCache cache.SemanticCache,
	embedder embedding.Embedder,
	exactCacheTTL time.Duration,
	semanticCacheTTL time.Duration,
	persistTurn ConversationTurnPersister,
	log logger.LoggerV1,
) *CacheWritebackNode {
	return &CacheWritebackNode{
		ExactCache:       exactCache,
		SemanticCache:    semanticCache,
		Embedder:         embedder,
		ExactCacheTTL:    exactCacheTTL,
		SemanticCacheTTL: semanticCacheTTL,
		PersistTurn:      persistTurn,
		Logger:           log,
	}
}

// Invoke 执行会话持久化和缓存写回。
func (n *CacheWritebackNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}

	resp := state.EnsureResponse()
	resp.Trace.TraceID = state.TraceID
	resp.Trace.CheckpointID = state.Checkpoint
	resp.Trace.CacheHit = state.Session.CacheHitLevel != ""
	resp.Trace.RewrittenQuery = state.Rewrite.Query

	if n.PersistTurn != nil {
		if err := n.PersistTurn(ctx, state, resp.Reply, resp.Intent, resp.Confidence); err != nil && n.Logger != nil {
			n.Logger.Warn("保存会话失败", logger.Error(err))
		}
	}

	if !n.shouldWriteCache(state, resp) {
		return state, nil
	}

	if n.ExactCache != nil && n.ExactCacheTTL > 0 {
		_ = n.ExactCache.Store(ctx, &cache.ExactCacheItem{
			TenantID: state.Session.TenantID,
			UserID:   state.Request.UserID,
			Query:    state.Session.RawQuery,
			Response: *resp,
		}, n.ExactCacheTTL)
	}

	if n.SemanticCache != nil && n.Embedder != nil && n.SemanticCacheTTL > 0 {
		intent := state.Session.Intent
		if intent == domain.IntentUnknown {
			intent = domain.IntentFallback
		}
		if support.AllowSemanticCache(intent, state.Session.RawQuery) {
			query := strings.TrimSpace(support.FirstNonEmpty(state.Rewrite.Query, state.Session.RawQuery))
			if query != "" {
				if vector, err := n.embedQuery(ctx, query); err == nil && len(vector) > 0 {
					scope := support.CacheScopeForIntent(intent)
					userID := int64(0)
					if scope == cache.CacheScopeTenantUser {
						userID = state.Request.UserID
					}
					_ = n.SemanticCache.Store(ctx, &cache.SemanticCacheItem{
						TenantID:     state.Session.TenantID,
						UserID:       userID,
						IntentBucket: support.CacheIntentBucket(intent),
						Scope:        scope,
						Query:        query,
						Response:     *resp,
						Vector:       vector,
					}, n.SemanticCacheTTL)
				}
			}
		}
	}

	return state, nil
}

func (n *CacheWritebackNode) shouldWriteCache(state *graphstate.ConversationState, resp *domain.ChatResult) bool {
	return resp != nil &&
		state.Session.ReadOnly &&
		state.Session.CacheHitLevel == "" &&
		!resp.NeedHandoff &&
		!state.Session.AwaitingUser &&
		!state.Session.AwaitingConfirm &&
		strings.TrimSpace(resp.Reply) != ""
}

func (n *CacheWritebackNode) embedQuery(ctx context.Context, query string) ([]float64, error) {
	vectors, err := n.Embedder.EmbedStrings(ctx, []string{query})
	if err != nil || len(vectors) == 0 {
		return nil, err
	}
	return vectors[0], nil
}
