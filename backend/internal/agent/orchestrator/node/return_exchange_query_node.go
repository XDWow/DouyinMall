package node

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// ReturnExchangeQueryInput 描述退换货申请预查询节点的输入。
type ReturnExchangeQueryInput struct {
	Slots map[string]any
}

// ReturnExchangeQueryNode 负责生成售后前置订单查询计划。
type ReturnExchangeQueryNode struct {
	RegistryHasTool ToolRegistryCheck
}

func NewReturnExchangeQueryNode(registryHasTool ToolRegistryCheck) *ReturnExchangeQueryNode {
	return &ReturnExchangeQueryNode{RegistryHasTool: registryHasTool}
}

// Invoke 完成售后前置订单查询计划构建。
func (n *ReturnExchangeQueryNode) Invoke(ctx context.Context, input ReturnExchangeQueryInput) (*ToolPlanResult, error) {
	result := &ToolPlanResult{}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "query_order") {
		result.FinalAnswer = "订单查询服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "order_service_unavailable"
		return result, nil
	}

	orderID, err := parseSlotInt64(input.Slots, "order_id")
	if err != nil {
		return nil, err
	}

	result.Plans = []domain.ToolCallPlan{{
		Name:      "query_order",
		Arguments: map[string]any{"order_id": orderID, "limit": 1},
		Reason:    "load_order_for_after_sale",
	}}
	return result, nil
}
