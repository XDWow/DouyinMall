package order

import (
	"context"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

type OrderReadInput struct {
	Slots map[string]any
}

type OrderReadNode struct {
	RegistryHasTool orchestratorshared.ToolRegistryCheck
}

func NewOrderReadNode(registryHasTool orchestratorshared.ToolRegistryCheck) *OrderReadNode {
	return &OrderReadNode{RegistryHasTool: registryHasTool}
}

func (n *OrderReadNode) Invoke(ctx context.Context, input OrderReadInput) (*orchestratorshared.ToolPlanResult, error) {
	result := &orchestratorshared.ToolPlanResult{ReadOnly: true}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "query_order") {
		result.FinalAnswer = "订单查询服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "order_service_unavailable"
		return result, nil
	}

	plans := []domain.ToolCallPlan{{Name: "query_order", Arguments: map[string]any{"limit": 5}}}
	if orderID := orchestratorshared.SlotString(input.Slots, "order_id"); orderID != "" {
		if value, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			plans[0].Arguments["order_id"] = value
			plans[0].Arguments["limit"] = 1
		}
	}

	result.Plans = plans
	return result, nil
}
