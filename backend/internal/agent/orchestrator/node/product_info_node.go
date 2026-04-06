package node

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// ProductInfoInput 描述商品咨询节点的输入。
type ProductInfoInput struct {
	Slots    map[string]any
	RawQuery string
}

// ProductInfoNode 负责生成商品咨询所需的工具调用计划。
type ProductInfoNode struct {
	RegistryHasTool ToolRegistryCheck
}

func NewProductInfoNode(registryHasTool ToolRegistryCheck) *ProductInfoNode {
	return &ProductInfoNode{RegistryHasTool: registryHasTool}
}

// Invoke 完成商品咨询前的计划构建。
func (n *ProductInfoNode) Invoke(ctx context.Context, input ProductInfoInput) (*ToolPlanResult, error) {
	result := &ToolPlanResult{ReadOnly: true}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "get_product") {
		result.FinalAnswer = "商品服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "product_service_unavailable"
		return result, nil
	}

	productID, err := parseSlotInt64(input.Slots, "product_id", "sku_id")
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
