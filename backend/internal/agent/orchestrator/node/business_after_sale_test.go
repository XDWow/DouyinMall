package node

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func TestBuildSubmitRequestClearsStaleToolPlansWhenNotConfirmed(t *testing.T) {
	suite := NewSuite(Dependencies{})
	node := suite.SubmitAfterSale()
	state := graphstate.NewConversationState(domain.ChatCommand{SessionID: "sess_1", UserID: 1, Message: "i want to return"}, nil, graphstate.InitOptions{})
	state.Tool.Plans = []domain.ToolCallPlan{{Name: "query_order"}}
	state.Tool.CallMessage = schema.AssistantMessage("", nil)
	if _, err := node.BuildRequest(context.Background(), state); err != nil {
		t.Fatalf("build submit request failed: %v", err)
	}
	if len(state.Tool.Plans) != 0 {
		t.Fatalf("expected stale tool plans to be cleared, got %+v", state.Tool.Plans)
	}
	if state.Tool.CallMessage != nil {
		t.Fatal("expected stale decision message to be cleared")
	}
}

func TestEligibilityCheckUsesServerSideIneligibleSignal(t *testing.T) {
	suite := NewSuite(Dependencies{})
	node := suite.EligibilityCheck()
	state := graphstate.NewConversationState(domain.ChatCommand{SessionID: "sess_2", UserID: 2, Message: "i want to return"}, nil, graphstate.InitOptions{})
	graphstate.SetSlot(state, "order_id", "10001")
	graphstate.SetSlot(state, "reason", "damaged")
	graphstate.SetSlot(state, "request_type", "return")
	state.Session.Slots["tool_results"] = map[string]any{"query_order": map[string]any{"eligible": false, "ineligible_reason": "after-sale window expired"}}
	if _, err := node.Invoke(context.Background(), state); err != nil {
		t.Fatalf("eligibility check failed: %v", err)
	}
	if state.Session.AwaitingConfirm {
		t.Fatal("expected ineligible order to stop before confirmation")
	}
	if state.Session.NeedHandoff {
		t.Fatal("expected explicit server-side ineligible result to avoid forced handoff")
	}
	if got := state.Session.FinalAnswer; got != "after-sale window expired" {
		t.Fatalf("unexpected final answer: %q", got)
	}
}

