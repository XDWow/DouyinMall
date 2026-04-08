package aftersale

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

type ReturnExchangeQueryInput struct {
	Slots map[string]any
}

type ReturnExchangeQueryNode struct {
	RegistryHasTool orchestratorshared.ToolRegistryCheck
}

func NewReturnExchangeQueryNode(registryHasTool orchestratorshared.ToolRegistryCheck) *ReturnExchangeQueryNode {
	return &ReturnExchangeQueryNode{RegistryHasTool: registryHasTool}
}

func (n *ReturnExchangeQueryNode) Invoke(ctx context.Context, input ReturnExchangeQueryInput) (*orchestratorshared.ToolPlanResult, error) {
	result := &orchestratorshared.ToolPlanResult{}
	if n.RegistryHasTool == nil || !n.RegistryHasTool(ctx, "query_order") {
		result.FinalAnswer = "订单查询服务暂时不可用，已为你转人工处理。"
		result.NeedHandoff = true
		result.HandoffReason = "order_service_unavailable"
		return result, nil
	}

	orderID, err := orchestratorshared.ParseSlotInt64(input.Slots, "order_id")
	if err != nil {
		return nil, err
	}

	result.Plans = []domain.ToolCallPlan{{
		Name:      "query_order",
		Arguments: map[string]any{"order_id": orderID, "limit": 1},
	}}
	result.ReadOnly = true
	return result, nil
}
