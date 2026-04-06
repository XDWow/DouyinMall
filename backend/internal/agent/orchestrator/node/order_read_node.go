package node

import (
	"context"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// OrderReadInput 描述订单查询节点的输入。
type OrderReadInput struct {
	Slots map[string]any
}

// OrderReadNode 负责生成订单查询所需的工具调用计划。
type OrderReadNode struct {
	RegistryHasTool ToolRegistryCheck
}

func NewOrderReadNode(registryHasTool ToolRegistryCheck) *OrderReadNode {
	return &OrderReadNode{RegistryHasTool: registryHasTool}
}

// Invoke 完成订单查询前的计划构建。
func (n *OrderReadNode) Invoke(ctx context.Context, input OrderReadInput) (*ToolPlanResult, error) {
	result := &ToolPlanResult{ReadOnly: true}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "query_order") {
		result.FinalAnswer = "订单查询服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "order_service_unavailable"
		return result, nil
	}

	plans := []domain.ToolCallPlan{{Name: "query_order", Arguments: map[string]any{"limit": 5}}}
	if orderID := slotString(input.Slots, "order_id"); orderID != "" {
		if value, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			plans[0].Arguments["order_id"] = value
			plans[0].Arguments["limit"] = 1
		}
	}
	result.Plans = plans
	return result, nil
}
