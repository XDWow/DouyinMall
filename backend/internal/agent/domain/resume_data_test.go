package domain

import "testing"

func TestParseApprovedFromResumeMap(t *testing.T) {
	t.Run("bool true", func(t *testing.T) {
		a, ok := ParseApprovedFromResumeMap(map[string]any{"approved": true})
		if !ok || !a {
			t.Fatalf("got %v %v", a, ok)
		}
	})
	t.Run("bool false", func(t *testing.T) {
		a, ok := ParseApprovedFromResumeMap(map[string]any{"approved": false})
		if !ok || a {
			t.Fatalf("got %v %v", a, ok)
		}
	})
	t.Run("string true", func(t *testing.T) {
		a, ok := ParseApprovedFromResumeMap(map[string]any{"approved": "true"})
		if !ok || !a {
			t.Fatalf("got %v %v", a, ok)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		_, ok := ParseApprovedFromResumeMap(map[string]any{})
		if ok {
			t.Fatal("expected not well-formed")
		}
	})
}

func TestResumeDataMergeFieldsIntoSlots(t *testing.T) {
	rd := ResumeDataFromMap(map[string]any{
		"fields": map[string]any{"product_id": "1001", "quantity": "2"},
	})
	slots := map[string]any{}
	rd.MergeFieldsIntoSlots(slots)
	if slots["quantity"] != "2" {
		t.Fatalf("quantity: %v", slots["quantity"])
	}
	if slots["product_id"] != "1001" {
		t.Fatalf("product_id: %v", slots["product_id"])
	}
}
