package global

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	und "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global/understanding"
)

type RouteNode struct{}

func NewRouteNode() *RouteNode { return &RouteNode{} }

func (n *RouteNode) Invoke(_ context.Context, in *und.UnderstandingResult) (domain.WorkflowRoute, error) {
	if in == nil {
		return domain.RouteUnknown, nil
	}
	return domain.WorkflowRouteFromIntent(domain.Intent(in.Intent)), nil
}
