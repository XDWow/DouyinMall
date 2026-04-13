package global

import (
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func TestResolveOrderRefFromTrustedRefs(t *testing.T) {
	slots := map[string]any{}
	intent := map[string]string{"order_ref": "current"}
	refs := domain.CurrentRefs{OrderID: "1000001"}
	ResolveOrderRefFromTrustedRefs(slots, intent, refs)
	if got := slots["order_id"]; got != "1000001" {
		t.Fatalf("order_id = %v, want 1000001", got)
	}
}

func TestResolveOrderRefFromTrustedRefsSkipsWhenOrderIDPresent(t *testing.T) {
	slots := map[string]any{"order_id": "99"}
	intent := map[string]string{"order_ref": "current"}
	refs := domain.CurrentRefs{OrderID: "1000001"}
	ResolveOrderRefFromTrustedRefs(slots, intent, refs)
	if got := slots["order_id"]; got != "99" {
		t.Fatalf("order_id = %v, want unchanged 99", got)
	}
}

func TestResolveOrderRefFromTrustedRefsNoOpWhenRefsEmpty(t *testing.T) {
	slots := map[string]any{}
	intent := map[string]string{"order_ref": "current"}
	ResolveOrderRefFromTrustedRefs(slots, intent, domain.CurrentRefs{})
	if len(slots) != 0 {
		t.Fatalf("expected empty slots, got %#v", slots)
	}
}
