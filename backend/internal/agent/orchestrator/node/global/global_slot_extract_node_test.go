package global

import (
	"context"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func TestGlobalSlotExtractNodeDoesNotTrustModelIDs(t *testing.T) {
	node := NewGlobalSlotExtractNode()

	result, err := node.Invoke(context.Background(), GlobalSlotExtractInput{
		Intent: domain.IntentProductInfo,
		IntentEntities: map[string]string{
			"product_id": "999999",
			"reason":     "只是看看",
		},
	})
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

func TestGlobalSlotExtractNodeUsesTrustedMetadataRefs(t *testing.T) {
	node := NewGlobalSlotExtractNode()

	result, err := node.Invoke(context.Background(), GlobalSlotExtractInput{
		Intent: domain.IntentOrderQuery,
		RequestMetadata: map[string]string{
			"order_id": "123456",
		},
		CurrentRefs: graphstate.CurrentRefs{
			OrderID: "100000",
		},
		IntentEntities: map[string]string{
			"order_id": "999999",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if got := result.CurrentRefs.OrderID; got != "123456" {
		t.Fatalf("current order id = %q, want 123456", got)
	}
	if got := result.Slots["order_id"]; got != "123456" {
		t.Fatalf("slot order_id = %v, want 123456", got)
	}
}
