package node

import (
	"context"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// AddToCartInput 描述加购节点的输入。
type AddToCartInput struct {
	Slots map[string]any
}

// AddToCartNode 负责生成加购所需的工具调用计划。
type AddToCartNode struct {
	RegistryHasTool ToolRegistryCheck
}

func NewAddToCartNode(registryHasTool ToolRegistryCheck) *AddToCartNode {
	return &AddToCartNode{RegistryHasTool: registryHasTool}
}

// Invoke 完成加购前的计划构建。
func (n *AddToCartNode) Invoke(ctx context.Context, input AddToCartInput) (*ToolPlanResult, error) {
	result := &ToolPlanResult{}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "add_to_cart") {
		result.FinalAnswer = "购物车服务暂时不可用，请稍后再试。"
		result.NeedHandoff = true
		result.HandoffReason = "cart_service_unavailable"
		return result, nil
	}

	productID, err := parseSlotInt64(input.Slots, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}

	quantity := int64(1)
	if raw := slotString(input.Slots, "quantity"); raw != "" {
		if q, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil && q > 0 {
			quantity = q
		}
	}

	result.Plans = []domain.ToolCallPlan{{
		Name:      "add_to_cart",
		Arguments: map[string]any{"product_id": productID, "quantity": quantity},
	}}
	return result, nil
}
