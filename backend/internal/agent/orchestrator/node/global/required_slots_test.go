package global

import (
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func TestRequiredMissingSlotsReasonInEntitiesNotSlots(t *testing.T) {
	slots := map[string]any{"order_id": "1"}
	entities := map[string]string{"reason": "不想要了"}
	missing := RequiredMissingSlots(domain.IntentReturnExchangeApply, slots, entities, false)
	if len(missing) != 0 {
		t.Fatalf("expected no missing, got %v", missing)
	}
}

func TestRequiredMissingSlotsReasonMissing(t *testing.T) {
	slots := map[string]any{"order_id": "1"}
	missing := RequiredMissingSlots(domain.IntentReturnExchangeApply, slots, nil, false)
	if len(missing) != 1 || missing[0] != "reason" {
		t.Fatalf("missing = %v, want [reason]", missing)
	}
}
