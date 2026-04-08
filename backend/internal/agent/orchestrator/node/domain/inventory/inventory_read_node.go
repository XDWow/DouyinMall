package inventory

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

type InventoryReadInput struct {
	Slots map[string]any
}

type InventoryReadNode struct {
	RegistryHasTool orchestratorshared.ToolRegistryCheck
}

func NewInventoryReadNode(registryHasTool orchestratorshared.ToolRegistryCheck) *InventoryReadNode {
	return &InventoryReadNode{RegistryHasTool: registryHasTool}
}

func (n *InventoryReadNode) Invoke(ctx context.Context, input InventoryReadInput) (*orchestratorshared.ToolPlanResult, error) {
	result := &orchestratorshared.ToolPlanResult{ReadOnly: true}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "get_inventory") {
		result.FinalAnswer = "库存查询服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "inventory_service_unavailable"
		return result, nil
	}

	productID, err := orchestratorshared.ParseSlotInt64(input.Slots, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}

	result.Plans = []domain.ToolCallPlan{{
		Name:      "get_inventory",
		Arguments: map[string]any{"product_id": productID},
	}}
	return result, nil
}
