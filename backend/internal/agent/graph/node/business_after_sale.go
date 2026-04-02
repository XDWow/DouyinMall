package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ReturnExchangeNode struct{ suite *Suite }

func (s *Suite) ReturnExchange() *ReturnExchangeNode { return &ReturnExchangeNode{suite: s} }

func (n *ReturnExchangeNode) BuildOrderQuery(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	support.ResetToolDecision(flow)
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "query_order") {
		flow.State.FinalAnswer = "Order query service is unavailable. Handing off to a human agent."
		flow.State.NeedHandoff = true
		flow.State.HandoffReason = "order_service_unavailable"
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	orderID, err := n.suite.ToolExec().ParseSlotInt64(flow, "order_id")
	if err != nil {
		return nil, err
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, flow, []dto.ToolCallPlan{{
		Name:      "query_order",
		Arguments: map[string]any{"order_id": orderID, "limit": 1},
		Reason:    "load_order_for_after_sale",
	}})
}

func (n *ReturnExchangeNode) ApplyOrderResult(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	support.HydrateToolResults(flow)
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *ReturnExchangeNode) EligibilityCheck(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	support.HydrateToolResults(flow)
	if flow.State.NeedHandoff {
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	if flow.State.AwaitingConfirm {
		switch {
		case support.IsNegative(flow.Request.Message):
			graphstate.SetSlot(flow, "confirm_status", "cancelled")
			flow.State.AwaitingConfirm = false
			flow.State.FinalAnswer = "The after-sale request has been cancelled."
		case support.IsAffirmative(flow.Request.Message):
			graphstate.SetSlot(flow, "confirm_status", "confirmed")
			flow.State.AwaitingConfirm = false
		default:
			graphstate.DeleteSlot(flow, "confirm_status")
			flow.State.AwaitingConfirm = true
			flow.State.FinalAnswer = support.BuildReturnApplySummary(flow)
		}
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	order := support.ToolResultRecord(flow, "query_order")
	if len(order) == 0 {
		flow.State.NeedHandoff = true
		flow.State.HandoffReason = "order_not_found"
		flow.State.FinalAnswer = "The order was not found. Please verify the order ID or hand off to a human agent."
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	if reply := buildAfterSaleIneligibleReply(flow, order); reply != "" {
		flow.State.AwaitingConfirm = false
		flow.State.ReadOnly = true
		flow.State.FinalAnswer = reply
		flow.State.NeedHandoff = false
		flow.State.HandoffReason = ""
		graphstate.DeleteSlot(flow, "confirm_status")
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	flow.State.AwaitingConfirm = true
	graphstate.DeleteSlot(flow, "confirm_status")
	flow.State.FinalAnswer = support.BuildReturnApplySummary(flow)
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *ReturnExchangeNode) ConfirmSummary(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	reply := strings.TrimSpace(flow.State.FinalAnswer)
	if reply == "" {
		reply = "Please confirm whether to continue the after-sale request."
	}
	resp := flow.EnsureResponse()
	resp.Reply = reply
	resp.Intent = flow.State.Intent
	resp.Status = dto.ReplyStatusFallback
	resp.Confidence = 0.9
	if n.suite.deps.Hooks.PersistConversationTurn != nil {
		if err := n.suite.deps.Hooks.PersistConversationTurn(ctx, flow, reply, resp.Intent, resp.Confidence); err != nil {
			n.suite.deps.Logger.Warn("persist confirm turn failed", logger.Error(err))
		}
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, compose.Interrupt(ctx, map[string]any{"confirm": true, "message": reply})
}

func (n *ReturnExchangeNode) BuildSubmitRequest(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	support.ResetToolDecision(flow)
	if !strings.EqualFold(graphstate.SlotString(flow, "confirm_status"), "confirmed") {
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	if n.suite.deps.Hooks.RegistryHasTool == nil || !n.suite.deps.Hooks.RegistryHasTool(ctx, "create_after_sale_request") {
		flow.State.NeedHandoff = true
		flow.State.HandoffReason = "after_sale_service_unavailable"
		flow.State.FinalAnswer = "After-sale submission is unavailable. Handing off to a human agent."
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	orderID, err := n.suite.ToolExec().ParseSlotInt64(flow, "order_id")
	if err != nil {
		return nil, err
	}
	args := map[string]any{
		"order_id":     orderID,
		"reason":       graphstate.SlotString(flow, "reason"),
		"request_type": support.FirstNonEmpty(graphstate.SlotString(flow, "request_type"), "return"),
	}
	if itemID := graphstate.SlotString(flow, "item_id", "sku_id", "product_id"); itemID != "" {
		if parsed, parseErr := n.suite.ToolExec().ParseSlotInt64(flow, "item_id", "sku_id", "product_id"); parseErr == nil {
			args["item_id"] = parsed
		}
	}
	return n.suite.deps.Hooks.ApplyToolPlans(ctx, flow, []dto.ToolCallPlan{{Name: "create_after_sale_request", Arguments: args, Reason: "submit_after_sale_request"}})
}

func (n *ReturnExchangeNode) SubmitAfterSale(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	if strings.EqualFold(graphstate.SlotString(flow, "confirm_status"), "cancelled") {
		flow.State.FinalAnswer = "The after-sale request has been cancelled."
		flow.State.AwaitingConfirm = false
		flow.State.ReadOnly = true
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	support.HydrateToolResults(flow)
	result := support.ToolResultMap(flow, "create_after_sale_request")
	if len(result) == 0 {
		flow.State.NeedHandoff = true
		flow.State.HandoffReason = "after_sale_submit_failed"
		flow.State.FinalAnswer = "After-sale submission failed. Handing off to a human agent."
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	requestNo := graphstate.ToString(result["request_no"])
	status := graphstate.ToString(result["status"])
	requestType := graphstate.ToString(result["request_type"])
	if requestType == "" {
		requestType = support.FirstNonEmpty(graphstate.SlotString(flow, "request_type"), "return")
	}
	flow.State.AwaitingConfirm = false
	flow.State.ReadOnly = true
	graphstate.DeleteSlot(flow, "confirm_status")
	flow.State.FinalAnswer = fmt.Sprintf("%s request submitted successfully. Request no: %s, status: %s.", support.AfterSaleTypeLabel(requestType), support.FirstNonEmpty(requestNo, "pending"), support.FirstNonEmpty(status, "pending_review"))
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func buildAfterSaleIneligibleReply(flow *graphstate.FlowContext, order map[string]any) string {
	if len(order) == 0 {
		return ""
	}
	reason := support.FirstNonEmpty(orderString(order, "after_sale_reason"), orderString(order, "ineligible_reason"), orderString(order, "reject_reason"), orderString(order, "reason"), orderString(order, "message"))
	if ok, exists := support.ToolResultBool(order, "eligible", "after_sale_eligible", "can_after_sale", "can_apply_after_sale"); exists && !ok {
		return support.FirstNonEmpty(reason, "This order does not meet after-sale requirements.")
	}
	if ok, exists := support.ToolResultBool(order, "in_after_sale_window", "within_after_sale_window"); exists && !ok {
		return support.FirstNonEmpty(reason, "This order is already outside the after-sale window.")
	}
	switch strings.ToLower(strings.TrimSpace(graphstate.SlotString(flow, "request_type"))) {
	case "exchange":
		if ok, exists := support.ToolResultBool(order, "support_exchange", "exchange_supported", "can_exchange"); exists && !ok {
			return support.FirstNonEmpty(reason, "This order does not support exchange.")
		}
	default:
		if ok, exists := support.ToolResultBool(order, "support_return", "return_supported", "can_return"); exists && !ok {
			return support.FirstNonEmpty(reason, "This order does not support return.")
		}
	}
	return ""
}

func orderString(order map[string]any, key string) string {
	if len(order) == 0 {
		return ""
	}
	value, ok := order[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(graphstate.ToString(value))
}
