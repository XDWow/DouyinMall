package node

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type ReturnExchangeQueryNode struct{ suite *Suite }

func (s *Suite) ReturnExchangeQuery() *ReturnExchangeQueryNode {
	return &ReturnExchangeQueryNode{suite: s}
}

func (n *ReturnExchangeQueryNode) BuildOrderQuery(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	support.ResetToolDecision(state)
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "query_order") {
		state.Session.FinalAnswer = "Order query service is unavailable. Handing off to a human agent."
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "order_service_unavailable"
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	orderID, err := n.suite.ToolExec().ParseSlotInt64(state, "order_id")
	if err != nil {
		return nil, err
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, state, []domain.ToolCallPlan{{
		Name:      "query_order",
		Arguments: map[string]any{"order_id": orderID, "limit": 1},
		Reason:    "load_order_for_after_sale",
	}})
}

func (n *ReturnExchangeQueryNode) ApplyOrderResult(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	support.HydrateToolResults(state)
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

