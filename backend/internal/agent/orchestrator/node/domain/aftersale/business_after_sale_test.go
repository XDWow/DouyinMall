package aftersale

import (
	"context"
	"testing"
)

func TestSubmitAfterSaleFallsBackToInputRequestType(t *testing.T) {
	node := NewSubmitAfterSaleNode()
	result, err := node.Invoke(context.Background(), SubmitAfterSaleInput{
		ConfirmStatus: "confirmed",
		RequestType:   "return",
		SubmitResult: map[string]any{
			"request_no": "AS123",
			"status":     "pending_review",
		},
	})
	if err != nil {
		t.Fatalf("submit after sale failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected submit result")
	}
	if result.NeedHandoff {
		t.Fatal("expected successful submit to avoid handoff")
	}
	if result.FinalAnswer == "" {
		t.Fatal("expected final answer to be generated")
	}
}

func TestEligibilityCheckUsesServerSideIneligibleSignal(t *testing.T) {
	node := NewEligibilityCheckNode()
	result, err := node.Invoke(context.Background(), EligibilityCheckInput{
		Message: "i want to return",
		Slots: map[string]any{
			"order_id":     "10001",
			"reason":       "damaged",
			"request_type": "return",
		},
		QueryOrderResult: map[string]any{
			"eligible":          false,
			"ineligible_reason": "after-sale window expired",
		},
	})
	if err != nil {
		t.Fatalf("eligibility check failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected eligibility result")
	}
	if result.AwaitingConfirm {
		t.Fatal("expected ineligible order to stop before confirmation")
	}
	if result.NeedHandoff {
		t.Fatal("expected explicit server-side ineligible result to avoid forced handoff")
	}
	if got := result.FinalAnswer; got != "after-sale window expired" {
		t.Fatalf("unexpected final answer: %q", got)
	}
}
