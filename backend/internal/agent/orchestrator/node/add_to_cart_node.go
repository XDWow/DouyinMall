package node

import (
	"context"
	"fmt"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type AddToCartNodeDeps struct {
	RegistryHasTool ToolRegistryCheck
	ApplyToolPlans  ToolPlanApplier
}

type AddToCartNode struct{ deps AddToCartNodeDeps }

func NewAddToCartNode(deps AddToCartNodeDeps) *AddToCartNode {
	return &AddToCartNode{deps: deps}
}

func (n *AddToCartNode) BuildRequest(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	if n.deps.RegistryHasTool == nil || !n.deps.RegistryHasTool(ctx, "add_to_cart") {
		state.Session.FinalAnswer = "Cart service is unavailable. Please try again later."
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "cart_service_unavailable"
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	productID, err := parseSlotInt64(state, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}
	quantity := int64(1)
	if raw := graphstate.SlotString(state, "quantity"); raw != "" {
		if q, err := strconv.ParseInt(raw, 10, 64); err == nil && q > 0 {
			quantity = q
		}
	}
	return n.deps.ApplyToolPlans(ctx, state, []domain.ToolCallPlan{{
		Name:      "add_to_cart",
		Arguments: map[string]any{"product_id": productID, "quantity": quantity},
	}})
}

func (n *AddToCartNode) ApplyResult(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	support.HydrateToolResults(state)
	result := support.ToolResultRecord(state, "add_to_cart")
	if ok, exists := support.ToolResultBool(result, "success"); exists && ok {
		productID := support.FirstNonEmpty(graphstate.SlotString(state, "product_id"), "unknown")
		quantity := int64(1)
		if raw := graphstate.SlotString(state, "quantity"); raw != "" {
			if q, err := strconv.ParseInt(raw, 10, 64); err == nil && q > 0 {
				quantity = q
			}
		}
		state.Session.FinalAnswer = fmt.Sprintf("Product %s (qty %d) has been added to your cart.", productID, quantity)
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
