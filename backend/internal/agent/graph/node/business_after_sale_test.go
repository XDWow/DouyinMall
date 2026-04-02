package node

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
)

func TestBuildSubmitRequestClearsStaleToolPlansWhenNotConfirmed(t *testing.T) {
	suite := NewSuite(Dependencies{})
	node := suite.ReturnExchange()
	flow := graphstate.NewFlowContext(dto.ChatRequest{SessionID: "sess_1", UserID: 1, Message: "i want to return"}, nil, graphstate.InitOptions{})
	flow.Tool.Plans = []dto.ToolCallPlan{{Name: "query_order"}}
	flow.Tool.DecisionMessage = schema.AssistantMessage("", nil)
	if _, err := node.BuildSubmitRequest(context.Background(), flow); err != nil {
		t.Fatalf("build submit request failed: %v", err)
	}
	if len(flow.Tool.Plans) != 0 {
		t.Fatalf("expected stale tool plans to be cleared, got %+v", flow.Tool.Plans)
	}
	if flow.Tool.DecisionMessage != nil {
		t.Fatal("expected stale decision message to be cleared")
	}
}

func TestEligibilityCheckUsesServerSideIneligibleSignal(t *testing.T) {
	suite := NewSuite(Dependencies{})
	node := suite.ReturnExchange()
	flow := graphstate.NewFlowContext(dto.ChatRequest{SessionID: "sess_2", UserID: 2, Message: "i want to return"}, nil, graphstate.InitOptions{})
	graphstate.SetSlot(flow, "order_id", "10001")
	graphstate.SetSlot(flow, "reason", "damaged")
	graphstate.SetSlot(flow, "request_type", "return")
	flow.State.Slots["tool_results"] = map[string]any{"query_order": map[string]any{"eligible": false, "ineligible_reason": "after-sale window expired"}}
	if _, err := node.EligibilityCheck(context.Background(), flow); err != nil {
		t.Fatalf("eligibility check failed: %v", err)
	}
	if flow.State.AwaitingConfirm {
		t.Fatal("expected ineligible order to stop before confirmation")
	}
	if flow.State.NeedHandoff {
		t.Fatal("expected explicit server-side ineligible result to avoid forced handoff")
	}
	if got := flow.State.FinalAnswer; got != "after-sale window expired" {
		t.Fatalf("unexpected final answer: %q", got)
	}
}
