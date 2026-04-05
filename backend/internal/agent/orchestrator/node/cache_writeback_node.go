package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type CacheWritebackNodeDeps struct {
	ExactCache  cache.ExactCache
	L0CacheTTL  time.Duration
	PersistTurn ConversationTurnPersister
	Logger      logger.LoggerV1
}

type CacheWritebackNode struct{ deps CacheWritebackNodeDeps }

func NewCacheWritebackNode(deps CacheWritebackNodeDeps) *CacheWritebackNode {
	return &CacheWritebackNode{deps: deps}
}

func (n *CacheWritebackNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	resp := state.EnsureResponse()
	resp.Trace.TraceID = state.TraceID
	resp.Trace.CheckpointID = state.Checkpoint
	resp.Trace.CacheHit = state.Session.CacheHitLevel != ""
	resp.Trace.RewrittenQuery = state.Session.RewrittenQuery
	if n.deps.PersistTurn != nil {
		if err := n.deps.PersistTurn(ctx, state, resp.Reply, resp.Intent, resp.Confidence); err != nil {
			n.deps.Logger.Warn("save session failed", logger.Error(err))
		}
	}
	if n.deps.ExactCache != nil && state.Session.ReadOnly && state.Session.CacheHitLevel == "" && !resp.NeedHandoff && !state.Session.AwaitingUser && !state.Session.AwaitingConfirm && strings.TrimSpace(resp.Reply) != "" {
		_ = n.deps.ExactCache.Store(ctx, &cache.ExactCacheItem{
			TenantID: state.Session.TenantID,
			UserID:   state.Request.UserID,
			Query:    state.Session.RawQuery,
			Response: *resp,
		}, n.deps.L0CacheTTL)
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
