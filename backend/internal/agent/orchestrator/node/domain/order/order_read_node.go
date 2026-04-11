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
	hasGet := n.RegistryHasTool != nil && n.RegistryHasTool(ctx, "get_order")
	hasList := n.RegistryHasTool != nil && (n.RegistryHasTool(ctx, "list_user_orders") || n.RegistryHasTool(ctx, "query_order"))

	if !hasGet && !hasList {
		result.FinalAnswer = "订单查询服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "order_service_unavailable"
		return result, nil
	}

	if orderID := orchestratorshared.SlotString(input.Slots, "order_id"); orderID != "" {
		if !hasGet {
			result.FinalAnswer = "当前环境不支持按订单号精确查询，已为你转人工处理。"
			result.NeedHandoff = true
			result.HandoffReason = "order_service_unavailable"
			return result, nil
		}
		if value, err := strconv.ParseInt(orderID, 10, 64); err == nil && value > 0 {
			result.Plans = []domain.ToolCallPlan{{
				Name:      "get_order",
				Arguments: map[string]any{"order_id": value},
			}}
			return result, nil
		}
	}

	if !hasList {
		result.FinalAnswer = "订单列表查询暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "order_service_unavailable"
		return result, nil
	}

	listName := "list_user_orders"
	if !n.RegistryHasTool(ctx, "list_user_orders") && n.RegistryHasTool(ctx, "query_order") {
		listName = "query_order"
	}
	result.Plans = []domain.ToolCallPlan{{Name: listName, Arguments: map[string]any{}}}
	return result, nil
}
