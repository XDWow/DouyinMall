package node

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type InventoryReadNode struct{ suite *Suite }

func (s *Suite) InventoryRead() *InventoryReadNode { return &InventoryReadNode{suite: s} }

func (n *InventoryReadNode) BuildQuery(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "get_inventory") {
		state.Session.FinalAnswer = "Inventory query service is unavailable. Handing off to a human agent."
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "inventory_service_unavailable"
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	productID, err := n.suite.ToolExec().ParseSlotInt64(state, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, state, []domain.ToolCallPlan{{Name: "get_inventory", Arguments: map[string]any{"product_id": productID}}})
}

func (n *InventoryReadNode) ApplyResult(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	support.HydrateToolResults(state)
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

