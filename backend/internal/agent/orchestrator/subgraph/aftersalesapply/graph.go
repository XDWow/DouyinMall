package aftersalesapply

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type ResolveInput struct {
	OrderID      string
	OrderRef     string
	Reason       string
	RequestType  string
	CurrentOrder string
	OrderList    []string
}

type Resolved struct {
	OrderID     string
	Reason      string
	RequestType string
}

func InputFromState(st *domain.State) (ResolveInput, error) {
	if st == nil || st.Session == nil {
		return ResolveInput{}, fmt.Errorf("state session is required")
	}
	return ResolveInput{
		OrderID:      sharednode.SlotString(st.Session.Slots, "order_id"),
		OrderRef:     sharednode.SlotString(st.Session.Slots, "order_ref"),
		Reason:       sharednode.SlotString(st.Session.Slots, "reason"),
		RequestType:  support.FirstNonEmpty(sharednode.SlotString(st.Session.Slots, "request_type"), "return"),
		CurrentOrder: strings.TrimSpace(st.Session.CurrentOrder),
		OrderList:    append([]string(nil), st.Session.OrderList...),
	}, nil
}

func Build(_ context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry) (compose.AnyGraph, error) {
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))
	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, 384)

	wf.AddLambdaNode("AftersalesApplyAssistNode",
		compose.InvokableLambda(func(ctx context.Context, in assistInput) (*domain.ChatResult, error) {
			return runAgentAssist(ctx, agent, in)
		}),
		compose.WithStatePreHandler(func(_ context.Context, in assistInput, st *domain.State) (assistInput, error) {
			return assistInputFromState(st)
		}),
	).AddDependency(compose.START)

	wf.AddLambdaNode("AftersalesApplyResolveNode",
		compose.InvokableLambda(resolveInvoke),
		compose.WithStatePreHandler(func(_ context.Context, in ResolveInput, st *domain.State) (ResolveInput, error) {
			return InputFromState(st)
		}),
	).AddDependency("AftersalesApplyAssistNode")

	wf.AddLambdaNode("AftersalesApplyAssistResultNode", compose.InvokableLambda(
		func(_ context.Context, in *domain.ChatResult) (*domain.ChatResult, error) {
			return in, nil
		},
	)).AddInput("AftersalesApplyAssistNode")

	wf.AddLambdaNode("AftersalesApplyEnsureArgsNode", compose.InvokableLambda(ensureArgs)).
		AddInput("AftersalesApplyResolveNode")

	wf.AddLambdaNode("AftersalesApplyConfirmNode", compose.InvokableLambda(confirm)).
		AddInput("AftersalesApplyEnsureArgsNode")

	wf.AddLambdaNode("AftersalesApplySubmitNode", compose.InvokableLambda(
		func(ctx context.Context, in Resolved) (*domain.ChatResult, error) {
			return submit(ctx, in, registry)
		},
	)).AddInput("AftersalesApplyConfirmNode")

	wf.AddBranch("AftersalesApplyAssistNode", compose.NewGraphBranch(
		func(_ context.Context, in *domain.ChatResult) (string, error) {
			if in != nil {
				return "AftersalesApplyAssistResultNode", nil
			}
			return "AftersalesApplyResolveNode", nil
		},
		map[string]bool{
			"AftersalesApplyResolveNode":      true,
			"AftersalesApplyAssistResultNode": true,
		},
	))

	wf.End().
		AddInput("AftersalesApplySubmitNode").
		AddInput("AftersalesApplyAssistResultNode")
	return wf, nil
}

func resolveInvoke(_ context.Context, in ResolveInput) (Resolved, error) {
	orderID := strings.TrimSpace(in.OrderID)
	if orderID == "" {
		orderID = subgraphcommon.ResolveSelection(in.OrderRef, in.CurrentOrder, in.OrderList)
	}
	if orderID == "" && strings.TrimSpace(in.OrderRef) == "" {
		orderID = strings.TrimSpace(in.CurrentOrder)
	}
	return Resolved{
		OrderID:     orderID,
		Reason:      strings.TrimSpace(in.Reason),
		RequestType: normalizedRequestType(in.RequestType),
	}, nil
}

func ensureArgs(ctx context.Context, resolved Resolved) (Resolved, error) {
	wasInterrupted, hasState, interrupted := compose.GetInterruptState[domain.AftersalesApplyInterruptState](ctx)
	isResumeTarget, hasData, resumeMap := compose.GetResumeContext[map[string]any](ctx)

	state := domain.AftersalesApplyInterruptState{
		OrderID:     resolved.OrderID,
		Reason:      resolved.Reason,
		RequestType: resolved.RequestType,
	}
	if wasInterrupted && hasState {
		state = interrupted
	}
	if wasInterrupted && !isResumeTarget {
		missing := missingApplyFields(state)
		state.MissingFields = missing
		return Resolved{}, compose.StatefulInterrupt(ctx, applyClarificationInfo(missing), state)
	}
	if hasData {
		rd := domain.ResumeDataFromMap(resumeMap)
		if v := strings.TrimSpace(rd.Fields["order"]); v != "" {
			state.OrderID = v
		}
		if v := strings.TrimSpace(rd.Fields["order_id"]); v != "" {
			state.OrderID = v
		}
		if v := strings.TrimSpace(rd.Fields["reason"]); v != "" {
			state.Reason = v
		}
	}

	missing := missingApplyFields(state)
	if len(missing) == 0 {
		return Resolved{
			OrderID:     strings.TrimSpace(state.OrderID),
			Reason:      strings.TrimSpace(state.Reason),
			RequestType: normalizedRequestType(state.RequestType),
		}, nil
	}

	state.MissingFields = missing
	return Resolved{}, compose.StatefulInterrupt(ctx, applyClarificationInfo(missing), state)
}

func confirm(ctx context.Context, resolved Resolved) (Resolved, error) {
	wasInterrupted, hasState, interrupted := compose.GetInterruptState[domain.AftersalesApplyInterruptState](ctx)
	isResumeTarget, hasData, resumeMap := compose.GetResumeContext[map[string]any](ctx)

	state := domain.AftersalesApplyInterruptState{
		OrderID:     resolved.OrderID,
		Reason:      resolved.Reason,
		RequestType: resolved.RequestType,
	}
	if wasInterrupted && hasState {
		state = interrupted
	}
	if wasInterrupted && !isResumeTarget {
		return Resolved{}, compose.StatefulInterrupt(ctx, confirmationInfo(state.OrderID, state.Reason), state)
	}
	if hasData {
		rd := domain.ResumeDataFromMap(resumeMap)
		if !rd.Approved {
			return Resolved{
				OrderID:     state.OrderID,
				Reason:      state.Reason,
				RequestType: "cancelled",
			}, nil
		}
		return Resolved{
			OrderID:     state.OrderID,
			Reason:      state.Reason,
			RequestType: normalizedRequestType(state.RequestType),
		}, nil
	}

	return Resolved{}, compose.StatefulInterrupt(ctx, confirmationInfo(state.OrderID, state.Reason), state)
}

func submit(ctx context.Context, resolved Resolved, registry *agenttool.Registry) (*domain.ChatResult, error) {
	result := &domain.ChatResult{Intent: domain.IntentAftersalesApply}
	if resolved.RequestType == "cancelled" {
		result.Reply = "Cancelled this aftersales request."
		return result, nil
	}

	if registry == nil || !registry.Has("create_after_sale_request") {
		result.Reply = "Aftersales service is currently unavailable. Please try again later."
		result.NeedHandoff = true
		result.HandoffReason = "aftersales_service_unavailable"
		return result, nil
	}

	callMessage, err := support.BuildToolCallMessage("create_after_sale_request", map[string]any{
		"order_id":     parseOrderID(resolved.OrderID),
		"reason":       resolved.Reason,
		"request_type": normalizedRequestType(resolved.RequestType),
	})
	if err != nil {
		return nil, err
	}

	toolsNode, err := registry.ToolsNode()
	if err != nil {
		return nil, err
	}
	if _, err := toolsNode.Invoke(ctx, callMessage); err != nil {
		return nil, err
	}

	record := subgraphcommon.LatestToolResultMap(ctx, "create_after_sale_request")
	requestNo := strings.TrimSpace(fmt.Sprint(record["request_no"]))
	status := strings.TrimSpace(fmt.Sprint(record["status"]))
	if requestNo != "" {
		result.Reply = fmt.Sprintf("Aftersales request submitted successfully. Request no: %s, status: %s.", requestNo, support.FirstNonEmpty(status, "pending_review"))
	} else {
		result.Reply = fmt.Sprintf("Submitted aftersales request for order %s.", resolved.OrderID)
	}

	_ = domain.ProcessState(ctx, func(st *domain.State) error {
		if st == nil || st.Session == nil {
			return nil
		}
		st.Session.CurrentOrder = strings.TrimSpace(resolved.OrderID)
		return nil
	})
	return result, nil
}

func missingApplyFields(st domain.AftersalesApplyInterruptState) []string {
	var missing []string
	if strings.TrimSpace(st.OrderID) == "" {
		missing = append(missing, "order")
	}
	if strings.TrimSpace(st.Reason) == "" {
		missing = append(missing, "reason")
	}
	return missing
}

func applyClarificationInfo(missing []string) map[string]any {
	question := "Please provide the missing information."
	if len(missing) > 0 {
		switch missing[0] {
		case "order":
			question = "Which order do you want to apply aftersales for?"
		case "reason":
			question = "What is the aftersales reason?"
		}
	}
	return map[string]any{
		"type":           "clarification",
		"question":       question,
		"missing_fields": append([]string(nil), missing...),
	}
}

func confirmationInfo(orderID, reason string) map[string]any {
	return map[string]any{
		"type":     "confirmation",
		"question": fmt.Sprintf("Please confirm submitting the aftersales request for order %s. Reason: %s.", orderID, reason),
	}
}

func parseOrderID(raw string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return v
}

func normalizedRequestType(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "exchange") {
		return "exchange"
	}
	return "return"
}
