package product

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type ProductInfoInput struct {
	Slots    map[string]any
	RawQuery string
}

type ProductInfoNode struct {
	RegistryHasTool orchestratorshared.ToolRegistryCheck
}

func NewProductInfoNode(registryHasTool orchestratorshared.ToolRegistryCheck) *ProductInfoNode {
	return &ProductInfoNode{RegistryHasTool: registryHasTool}
}

func (n *ProductInfoNode) Invoke(ctx context.Context, input ProductInfoInput) (*orchestratorshared.ToolPlanResult, error) {
	result := &orchestratorshared.ToolPlanResult{ReadOnly: true}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "get_product") {
		result.FinalAnswer = "商品服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "product_service_unavailable"
		return result, nil
	}

	productID, err := orchestratorshared.ParseSlotInt64(input.Slots, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}

	plans := []domain.ToolCallPlan{{Name: "get_product", Arguments: map[string]any{"product_id": productID}}}
	if n.RegistryHasTool != nil && n.RegistryHasTool(ctx, "get_inventory") && support.MentionsInventory(input.RawQuery) {
		plans = append(plans, domain.ToolCallPlan{Name: "get_inventory", Arguments: map[string]any{"product_id": productID}})
	}
	result.Plans = plans
	return result, nil
}
