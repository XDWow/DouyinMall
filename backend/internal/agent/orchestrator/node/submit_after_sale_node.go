package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type SubmitAfterSaleNodeDeps struct {
	RegistryHasTool ToolRegistryCheck
	ApplyToolPlans  ToolPlanApplier
}

type SubmitAfterSaleNode struct{ deps SubmitAfterSaleNodeDeps }

func NewSubmitAfterSaleNode(deps SubmitAfterSaleNodeDeps) *SubmitAfterSaleNode {
	return &SubmitAfterSaleNode{deps: deps}
}

func (n *SubmitAfterSaleNode) BuildRequest(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	support.ResetToolDecision(state)
	if !strings.EqualFold(graphstate.SlotString(state, "confirm_status"), "confirmed") {
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	if n.deps.RegistryHasTool == nil || !n.deps.RegistryHasTool(ctx, "create_after_sale_request") {
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "after_sale_service_unavailable"
		state.Session.FinalAnswer = "After-sale submission is unavailable. Handing off to a human agent."
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	orderID, err := parseSlotInt64(state, "order_id")
	if err != nil {
		return nil, err
	}
	args := map[string]any{
		"order_id":     orderID,
		"reason":       graphstate.SlotString(state, "reason"),
		"request_type": support.FirstNonEmpty(graphstate.SlotString(state, "request_type"), "return"),
	}
	if graphstate.SlotString(state, "item_id", "sku_id", "product_id") != "" {
		if parsed, parseErr := parseSlotInt64(state, "item_id", "sku_id", "product_id"); parseErr == nil {
			args["item_id"] = parsed
		}
	}
	return n.deps.ApplyToolPlans(ctx, state, []domain.ToolCallPlan{{Name: "create_after_sale_request", Arguments: args, Reason: "submit_after_sale_request"}})
}

func (n *SubmitAfterSaleNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	if strings.EqualFold(graphstate.SlotString(state, "confirm_status"), "cancelled") {
		state.Session.FinalAnswer = "The after-sale request has been cancelled."
		state.Session.AwaitingConfirm = false
		state.Session.ReadOnly = true
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	support.HydrateToolResults(state)
	result := support.ToolResultMap(state, "create_after_sale_request")
	if len(result) == 0 {
		state.Session.NeedHandoff = true
		state.Session.HandoffReason = "after_sale_submit_failed"
		state.Session.FinalAnswer = "After-sale submission failed. Handing off to a human agent."
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	requestNo := graphstate.ToString(result["request_no"])
	status := graphstate.ToString(result["status"])
	requestType := graphstate.ToString(result["request_type"])
	if requestType == "" {
		requestType = support.FirstNonEmpty(graphstate.SlotString(state, "request_type"), "return")
	}
	state.Session.AwaitingConfirm = false
	state.Session.ReadOnly = true
	graphstate.DeleteSlot(state, "confirm_status")
	state.Session.FinalAnswer = fmt.Sprintf("%s request submitted successfully. Request no: %s, status: %s.", support.AfterSaleTypeLabel(requestType), support.FirstNonEmpty(requestNo, "pending"), support.FirstNonEmpty(status, "pending_review"))
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
