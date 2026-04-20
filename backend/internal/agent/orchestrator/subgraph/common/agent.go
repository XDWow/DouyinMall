package common

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

const defaultClarificationQuestion = "Please provide more detail."

type AgentDecision struct {
	Type          string   `json:"type"`
	Reply         string   `json:"reply,omitempty"`
	Question      string   `json:"question,omitempty"`
	MissingFields []string `json:"missing_fields,omitempty"`
	NeedHandoff   bool     `json:"need_handoff,omitempty"`
	HandoffReason string   `json:"handoff_reason,omitempty"`
}

func HistoryMessages(turns []domain.MessageTurn) []*schema.Message {
	if len(turns) == 0 {
		return nil
	}

	out := make([]*schema.Message, 0, len(turns))
	for _, turn := range turns {
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		switch turn.Role {
		case domain.RoleAssistant:
			out = append(out, schema.AssistantMessage(content, nil))
		case domain.RoleSystem:
			out = append(out, schema.SystemMessage(content))
		default:
			out = append(out, schema.UserMessage(content))
		}
	}
	return out
}

func ParseAgentDecision(raw string) AgentDecision {
	cleaned := support.CleanJSON(strings.TrimSpace(raw))
	if cleaned != "" {
		var decision AgentDecision
		if err := json.Unmarshal([]byte(cleaned), &decision); err == nil {
			decision.Type = strings.TrimSpace(strings.ToLower(decision.Type))
			decision.Reply = strings.TrimSpace(decision.Reply)
			decision.Question = strings.TrimSpace(decision.Question)
			return normalizeDecision(decision)
		}
	}

	return AgentDecision{
		Type:  "answer",
		Reply: strings.TrimSpace(raw),
	}
}

func InterruptForDecision(ctx context.Context, decision AgentDecision) error {
	return compose.Interrupt(ctx, map[string]any{
		"type":           "clarification",
		"question":       support.FirstNonEmpty(decision.Question, defaultClarificationQuestion),
		"missing_fields": append([]string(nil), decision.MissingFields...),
	})
}

func normalizeDecision(decision AgentDecision) AgentDecision {
	switch decision.Type {
	case "clarify", "clarification":
		decision.Type = "clarification"
		if decision.Question == "" {
			decision.Question = defaultClarificationQuestion
		}
	default:
		decision.Type = "answer"
	}
	return decision
}

func RenderSlotsContext(slots map[string]any) string {
	if len(slots) == 0 {
		return ""
	}

	keys := make([]string, 0, len(slots))
	for key, value := range slots {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		if text := strings.TrimSpace(fmt.Sprint(value)); text == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}

	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+strings.TrimSpace(fmt.Sprint(slots[key])))
	}
	return strings.Join(lines, "\n")
}
