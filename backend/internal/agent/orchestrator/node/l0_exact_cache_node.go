package node

import (
	"context"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type L0ExactCacheNode struct{ suite *Suite }

func (s *Suite) L0ExactCache() *L0ExactCacheNode { return &L0ExactCacheNode{suite: s} }

func (n *L0ExactCacheNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil || n.suite.deps.ExactCache == nil || state.Session.ResumeFromCP {
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	item, err := n.suite.deps.ExactCache.Lookup(ctx, state.Session.TenantID, state.Request.UserID, state.Session.RawQuery)
	if err != nil || item == nil {
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	resp := item.Response
	resp.Trace.TraceID = state.TraceID
	resp.Trace.CheckpointID = state.Checkpoint
	resp.Trace.CacheHit = true
	resp.SessionID = state.Request.SessionID
	state.Response = &resp
	state.Session.CacheHitLevel = "L0"
	state.Session.FinalAnswer = resp.Reply
	state.Session.Intent = resp.Intent
	state.Session.Route = support.RouteFromIntent(resp.Intent)
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

