package global

import (
	"context"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

// RouteInput 鎻忚堪涓氬姟璺敱闃舵鐨勮緭鍏ャ€?
type RouteInput struct {
	Intent          graphstate.IntentResult
	FeatureFlags    graphstate.FeatureFlags
	AwaitingConfirm bool
}

// RouteNode 璐熻矗鏍规嵁鎰忓浘鍜屽姛鑳藉紑鍏抽€夋嫨鍚庣画涓氬姟瀛愬浘銆?
type RouteNode struct{}

func NewRouteNode() *RouteNode { return &RouteNode{} }

type RouteResult struct {
	Route     graphstate.WorkflowRoute
	ErrorCode string
	ReadOnly  bool
}

// Invoke 璁＄畻褰撳墠璇锋眰鐨勪笟鍔¤矾鐢便€?
func (n *RouteNode) Invoke(_ context.Context, input RouteInput) (*RouteResult, error) {
	route := routeFromIntent(input.Intent.Intent)
	if input.AwaitingConfirm {
		route = graphstate.RouteReturnExchangeApply
	}

	errorCode := ""
	if !routeEnabled(input.FeatureFlags, route) {
		route = graphstate.RouteBaseQA
		errorCode = "feature_disabled"
	}

	return &RouteResult{
		Route:     route,
		ErrorCode: errorCode,
		ReadOnly:  route != graphstate.RouteReturnExchangeApply && route != graphstate.RouteAddToCart,
	}, nil
}
