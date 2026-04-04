package node

import (
	"context"
	"fmt"
	"strings"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type EligibilityCheckNode struct{ suite *Suite }

func (s *Suite) EligibilityCheck() *EligibilityCheckNode { return &EligibilityCheckNode{suite: s} }

func (n *EligibilityCheckNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	support.HydrateToolResults(state)
	if state.Session.NeedHandoff {
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	if state.Session.AwaitingConfirm {
		switch {
		case support.IsNegative(state.Request.Message):
			graphstate.SetSlot(state, "confirm_status", "cancelled")
			state.Session.AwaitingConfirm = false
			state.Session.FinalAnswer = "The after-sale request has been cancelled."
		case support.IsAffirmative(state.Request.Message):
			graphstate.SetSlot(state, "confirm_status", "confirmed")
			state.Session.AwaitingConfirm = false
		default:
			graphstate.DeleteSlot(state, "confirm_status")
			state.Session.AwaitingConfirm = true
			state.Session.FinalAnswer = support.BuildReturnApplySummary(state)
		}
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	order := support.ToolResultRecord(state, "query_order")
	if len(order) == 0 {
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "order_not_found"
		state.Session.FinalAnswer = "The order was not found. Please verify the order ID or hand off to a human agent."
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	if reply := buildAfterSaleIneligibleReply(state, order); reply != "" {
		state.Session.AwaitingConfirm = false
		state.Session.ReadOnly = true
		state.Session.FinalAnswer = reply
		state.Session.NeedHandoff = false
		state.Session.HandoffReason = ""
		graphstate.DeleteSlot(state, "confirm_status")
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	state.Session.AwaitingConfirm = true
	graphstate.DeleteSlot(state, "confirm_status")
	state.Session.FinalAnswer = support.BuildReturnApplySummary(state)
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

func buildAfterSaleIneligibleReply(state *graphstate.ConversationState, order map[string]any) string {
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
	switch strings.ToLower(strings.TrimSpace(graphstate.SlotString(state, "request_type"))) {
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

