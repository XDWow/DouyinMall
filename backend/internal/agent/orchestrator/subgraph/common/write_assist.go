package common

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type WriteAssistDecision struct {
	Mode          string         `json:"mode"`
	Reply         string         `json:"reply,omitempty"`
	Question      string         `json:"question,omitempty"`
	MissingFields []string       `json:"missing_fields,omitempty"`
	NeedHandoff   bool           `json:"need_handoff,omitempty"`
	HandoffReason string         `json:"handoff_reason,omitempty"`
	SlotsPatch    map[string]any `json:"slots_patch,omitempty"`
}

func ParseWriteAssistDecision(raw string) WriteAssistDecision {
	cleaned := support.CleanJSON(strings.TrimSpace(raw))
	if cleaned != "" {
		var decision WriteAssistDecision
		if err := json.Unmarshal([]byte(cleaned), &decision); err == nil {
			decision.Mode = strings.TrimSpace(strings.ToLower(decision.Mode))
			decision.Reply = strings.TrimSpace(decision.Reply)
			decision.Question = strings.TrimSpace(decision.Question)
			decision.HandoffReason = strings.TrimSpace(decision.HandoffReason)
			return normalizeWriteAssistDecision(decision)
		}
	}

	return WriteAssistDecision{
		Mode:     "clarification",
		Question: "Please provide the missing information.",
	}
}

func InterruptForWriteAssist(ctx context.Context, decision WriteAssistDecision) error {
	return compose.Interrupt(ctx, map[string]any{
		"type":           "clarification",
		"question":       support.FirstNonEmpty(decision.Question, "Please provide the missing information."),
		"missing_fields": append([]string(nil), decision.MissingFields...),
	})
}

func ApplySlotsPatch(ctx context.Context, patch map[string]any) error {
	if len(patch) == 0 {
		return nil
	}
	return domain.ProcessState(ctx, func(st *domain.State) error {
		if st == nil {
			return nil
		}
		if st.Session == nil {
			st.Session = &domain.Session{}
		}
		if st.Session.Slots == nil {
			st.Session.Slots = make(map[string]any)
		}
		support.MergeSlots(st.Session.Slots, patch)
		return nil
	})
}

func normalizeWriteAssistDecision(decision WriteAssistDecision) WriteAssistDecision {
	switch decision.Mode {
	case "ready", "continue", "proceed":
		decision.Mode = "ready"
	case "answer":
		decision.Mode = "answer"
	case "handoff":
		decision.Mode = "handoff"
		decision.NeedHandoff = true
	case "clarify", "clarification":
		decision.Mode = "clarification"
		if decision.Question == "" {
			decision.Question = "Please provide the missing information."
		}
	default:
		decision.Mode = "clarification"
		if decision.Question == "" {
			decision.Question = "Please provide the missing information."
		}
	}
	return decision
}
