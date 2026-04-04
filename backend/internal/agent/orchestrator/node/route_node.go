package node

import (
	"context"
	"fmt"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type RouteNode struct{ suite *Suite }

func (s *Suite) Route() *RouteNode { return &RouteNode{suite: s} }

func (n *RouteNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	ss := graphstate.EnsureSessionState(state)
	route := support.RouteFromIntent(ss.Intent)
	if ss.AwaitingConfirm {
		route = graphstate.RouteReturnExchangeApply
	}
	if !support.RouteEnabled(ss.FeatureFlags, route) {
		route = graphstate.RouteFallback
		ss.ErrorCode = "feature_disabled"
	}
	ss.Route = route
	ss.ReadOnly = route != graphstate.RouteReturnExchangeApply
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
