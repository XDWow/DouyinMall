package node

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type ProductInfoNode struct{ suite *Suite }

func (s *Suite) ProductInfo() *ProductInfoNode { return &ProductInfoNode{suite: s} }

func (n *ProductInfoNode) BuildQuery(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "get_product") {
		state.Session.FinalAnswer = "Product service is unavailable. Handing off to a human agent."
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "product_service_unavailable"
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	productID, err := n.suite.ToolExec().ParseSlotInt64(state, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}
	plans := []domain.ToolCallPlan{{Name: "get_product", Arguments: map[string]any{"product_id": productID}}}
	if n.suite.deps.Hooks.RegistryHasTool != nil && n.suite.deps.Hooks.RegistryHasTool(ctx, "get_inventory") && support.MentionsInventory(state.Session.RawQuery) {
		plans = append(plans, domain.ToolCallPlan{Name: "get_inventory", Arguments: map[string]any{"product_id": productID}})
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, state, plans)
}

func (n *ProductInfoNode) ApplyResult(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	support.HydrateToolResults(state)
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

