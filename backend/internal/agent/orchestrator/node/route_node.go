package node

import (
	"context"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// RouteInput 描述业务路由阶段的输入。
type RouteInput struct {
	Intent          graphstate.IntentResult
	FeatureFlags    graphstate.FeatureFlags
	AwaitingConfirm bool
}

// RouteNode 负责根据意图和功能开关选择后续业务子图。
type RouteNode struct{}

func NewRouteNode() *RouteNode { return &RouteNode{} }

type RouteResult struct {
	Route     graphstate.WorkflowRoute
	ErrorCode string
	ReadOnly  bool
}

// Invoke 计算当前请求的业务路由。
func (n *RouteNode) Invoke(_ context.Context, input RouteInput) (*RouteResult, error) {
	route := support.RouteFromIntent(input.Intent.Intent)
	if input.AwaitingConfirm {
		route = graphstate.RouteReturnExchangeApply
	}

	errorCode := ""
	if !support.RouteEnabled(input.FeatureFlags, route) {
		route = graphstate.RouteFallback
		errorCode = "feature_disabled"
	}

	return &RouteResult{
		Route:     route,
		ErrorCode: errorCode,
		ReadOnly:  route != graphstate.RouteReturnExchangeApply && route != graphstate.RouteAddToCart,
	}, nil
}
