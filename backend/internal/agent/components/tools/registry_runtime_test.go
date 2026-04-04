package tool

import (
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func TestRegistryValidatePlansParallelReadOnly(t *testing.T) {
	registry := &Registry{
		invokables: map[string]registeredInvokableTool{
			"get_product": {
				policy: ToolPolicy{ReadOnly: true},
			},
			"get_inventory": {
				policy: ToolPolicy{ReadOnly: true},
			},
		},
	}

	plans := []domain.ToolCallPlan{
		{Name: "get_product", RawJSON: `{"product_id":1}`},
		{Name: "get_inventory", RawJSON: `{"product_id":1}`},
	}
	if err := registry.ValidatePlans(plans, ToolExecutionParallelReadOnly); err != nil {
		t.Fatalf("validate plans failed: %v", err)
	}
}

func TestRegistryValidatePlansParallelReadOnlyRejectsWriteTool(t *testing.T) {
	registry := &Registry{
		invokables: map[string]registeredInvokableTool{
			"add_to_cart": {
				policy: ToolPolicy{ReadOnly: false, RequiresOrdering: true},
			},
		},
	}

	plans := []domain.ToolCallPlan{{Name: "add_to_cart", RawJSON: `{"product_id":1}`}}
	if err := registry.ValidatePlans(plans, ToolExecutionParallelReadOnly); err == nil {
		t.Fatal("expected parallel readonly validation to reject write tool")
	}
}
