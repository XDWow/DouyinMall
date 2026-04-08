package cart

import (
	"context"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

type AddToCartInput struct {
	Slots map[string]any
}

type AddToCartNode struct {
	RegistryHasTool orchestratorshared.ToolRegistryCheck
}

func NewAddToCartNode(registryHasTool orchestratorshared.ToolRegistryCheck) *AddToCartNode {
	return &AddToCartNode{RegistryHasTool: registryHasTool}
}

func (n *AddToCartNode) Invoke(ctx context.Context, input AddToCartInput) (*orchestratorshared.ToolPlanResult, error) {
	result := &orchestratorshared.ToolPlanResult{}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "add_to_cart") {
		result.FinalAnswer = "购物车服务暂时不可用，请稍后再试。"
		result.NeedHandoff = true
		result.HandoffReason = "cart_service_unavailable"
		return result, nil
	}

	productID, err := orchestratorshared.ParseSlotInt64(input.Slots, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}

	quantity := int64(1)
	if raw := orchestratorshared.SlotString(input.Slots, "quantity"); raw != "" {
		if parsed, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil && parsed > 0 {
			quantity = parsed
		}
	}

	result.Plans = []domain.ToolCallPlan{{
		Name: "add_to_cart",
		Arguments: map[string]any{
			"product_id": productID,
			"quantity":   quantity,
		},
	}}
	result.ReadOnly = false
	return result, nil
}
