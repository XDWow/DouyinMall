package global

import (
	"context"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func TestSlotMergeNodeDoesNotTrustModelIDs(t *testing.T) {
	node := NewSlotMergeNode()

	result, err := node.Invoke(context.Background(), SlotMergeInput{})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if got := result.Slots["product_id"]; got != nil {
		t.Fatalf("product_id = %v, want nil", got)
	}
	if got := result.CurrentRefs.ProductID; got != "" {
		t.Fatalf("current product id = %q, want empty", got)
	}
}

func TestSlotMergeNodeIgnoresRequestMetadataUsesCurrentRefsOnly(t *testing.T) {
	node := NewSlotMergeNode()

	result, err := node.Invoke(context.Background(), SlotMergeInput{
		CurrentRefs: domain.CurrentRefs{
			OrderID: "100000",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if got := result.CurrentRefs.OrderID; got != "100000" {
		t.Fatalf("current order id = %q, want 100000", got)
	}
	if got := result.Slots["order_id"]; got != "100000" {
		t.Fatalf("slot order_id = %v, want 100000", got)
	}
}
