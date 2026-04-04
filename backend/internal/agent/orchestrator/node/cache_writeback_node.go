package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type CacheWritebackNode struct{ suite *Suite }

func (s *Suite) CacheWriteback() *CacheWritebackNode { return &CacheWritebackNode{suite: s} }

func (n *CacheWritebackNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	resp := state.EnsureResponse()
	resp.Trace.TraceID = state.TraceID
	resp.Trace.CheckpointID = state.Checkpoint
	resp.Trace.CacheHit = state.Session.CacheHitLevel != ""
	resp.Trace.RewrittenQuery = state.Session.RewrittenQuery
	if n.suite.deps.Hooks.PersistConversationTurn != nil {
		if err := n.suite.deps.Hooks.PersistConversationTurn(ctx, state, resp.Reply, resp.Intent, resp.Confidence); err != nil {
			n.suite.deps.Logger.Warn("save session failed", logger.Error(err))
		}
	}
	if n.suite.deps.ExactCache != nil && state.Session.ReadOnly && state.Session.CacheHitLevel == "" && !resp.NeedHandoff && !state.Session.AwaitingUser && !state.Session.AwaitingConfirm && strings.TrimSpace(resp.Reply) != "" {
		_ = n.suite.deps.ExactCache.Store(ctx, &cache.ExactCacheItem{
			TenantID: state.Session.TenantID,
			UserID:   state.Request.UserID,
			Query:    state.Session.RawQuery,
			Response: *resp,
		}, n.suite.deps.Config.L0CacheTTL)
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

