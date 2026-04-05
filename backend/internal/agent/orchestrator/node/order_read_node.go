package node

import (
	"context"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type OrderReadNodeDeps struct {
	RegistryHasTool ToolRegistryCheck
	ApplyToolPlans  ToolPlanApplier
}

type OrderReadNode struct{ deps OrderReadNodeDeps }

func NewOrderReadNode(deps OrderReadNodeDeps) *OrderReadNode {
	return &OrderReadNode{deps: deps}
}

func (n *OrderReadNode) BuildQuery(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	state.Session.ReadOnly = true
	if n.deps.RegistryHasTool == nil || !n.deps.RegistryHasTool(ctx, "query_order") {
		state.Session.FinalAnswer = "Order query service is unavailable. Handing off to a human agent."
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "order_service_unavailable"
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	plans := []domain.ToolCallPlan{{Name: "query_order", Arguments: map[string]any{"limit": 5}}}
	if orderID := graphstate.SlotString(state, "order_id"); orderID != "" {
		if value, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			plans[0].Arguments["order_id"] = value
			plans[0].Arguments["limit"] = 1
		}
	}
	return n.deps.ApplyToolPlans(ctx, state, plans)
}

func (n *OrderReadNode) ApplyResult(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	support.HydrateToolResults(state)
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
