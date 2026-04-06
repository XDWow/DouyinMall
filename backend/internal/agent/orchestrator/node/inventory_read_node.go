package node

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// InventoryReadInput 描述库存查询节点的输入。
type InventoryReadInput struct {
	Slots map[string]any
}

// InventoryReadNode 负责生成库存查询所需的工具调用计划。
type InventoryReadNode struct {
	RegistryHasTool ToolRegistryCheck
}

func NewInventoryReadNode(registryHasTool ToolRegistryCheck) *InventoryReadNode {
	return &InventoryReadNode{RegistryHasTool: registryHasTool}
}

// Invoke 完成库存查询前的计划构建。
func (n *InventoryReadNode) Invoke(ctx context.Context, input InventoryReadInput) (*ToolPlanResult, error) {
	result := &ToolPlanResult{ReadOnly: true}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "get_inventory") {
		result.FinalAnswer = "库存查询服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "inventory_service_unavailable"
		return result, nil
	}

	productID, err := parseSlotInt64(input.Slots, "product_id", "sku_id")
	if err != nil {
		return nil, err
	}

	result.Plans = []domain.ToolCallPlan{{
		Name:      "get_inventory",
		Arguments: map[string]any{"product_id": productID},
	}}
	return result, nil
}
