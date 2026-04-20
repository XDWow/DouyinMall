package common

import "testing"

func TestParseWriteAssistDecisionReady(t *testing.T) {
	decision := ParseWriteAssistDecision(`{
		"mode":"ready",
		"slots_patch":{"order_id":"1001","reason":"broken"}
	}`)
	if decision.Mode != "ready" {
		t.Fatalf("expected ready mode, got %q", decision.Mode)
	}
	if decision.SlotsPatch["order_id"] != "1001" {
		t.Fatalf("expected order_id patch to be preserved")
	}
}

func TestParseWriteAssistDecisionFallbackToClarification(t *testing.T) {
	decision := ParseWriteAssistDecision(`not-json`)
	if decision.Mode != "clarification" {
		t.Fatalf("expected clarification mode, got %q", decision.Mode)
	}
	if decision.Question == "" {
		t.Fatal("expected fallback clarification question")
	}
}
