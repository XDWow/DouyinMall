package global

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// RouteInput 显式入参（主图 Lambda 从 state 摘出）。
type RouteInput struct {
	Intent          domain.Intent
	AwaitingConfirm bool
}

// RouteNode Intent → WorkflowRoute。
type RouteNode struct{}

func NewRouteNode() *RouteNode { return &RouteNode{} }

type RouteResult struct {
	Route     domain.WorkflowRoute
	ErrorCode string
	ReadOnly  bool
}

func (n *RouteNode) Invoke(_ context.Context, input RouteInput) (*RouteResult, error) {
	route := routeFromIntent(input.Intent)
	if input.AwaitingConfirm {
		route = domain.RouteReturnExchangeApply
	}

	return &RouteResult{
		Route:     route,
		ErrorCode: "",
		ReadOnly:  route != domain.RouteReturnExchangeApply && route != domain.RouteAddToCart,
	}, nil
}
