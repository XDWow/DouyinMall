package observe

import (
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func TestEnrichTraceSlowest(t *testing.T) {
	resp := &domain.ChatResult{
		Trace: domain.Trace{
			Steps: []domain.TraceStep{
				{Node: "A", LatencyMs: 10},
				{Node: "B", LatencyMs: 80},
				{Node: "C", LatencyMs: 50},
			},
		},
	}
	EnrichTraceSlowest(resp)
	if resp.Trace.SlowestStepNode != "B" {
		t.Fatalf("SlowestStepNode = %q, want B", resp.Trace.SlowestStepNode)
	}
	if resp.Trace.SlowestStepLatencyMs != 80 {
		t.Fatalf("SlowestStepLatencyMs = %d, want 80", resp.Trace.SlowestStepLatencyMs)
	}
}

func TestEnrichTraceSlowestEmpty(t *testing.T) {
	resp := &domain.ChatResult{Trace: domain.Trace{Steps: nil}}
	EnrichTraceSlowest(resp)
	if resp.Trace.SlowestStepNode != "" || resp.Trace.SlowestStepLatencyMs != 0 {
		t.Fatalf("expected empty slowest, got %+v", resp.Trace)
	}
}
