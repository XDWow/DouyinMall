package toolexec

import (
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func CreateDecisionMessage(plans []domain.ToolCallPlan) (*schema.Message, error) {
	if len(plans) == 0 {
		return nil, nil
	}
	toolCalls := make([]schema.ToolCall, 0, len(plans))
	for _, plan := range plans {
		rawJSON := strings.TrimSpace(plan.RawJSON)
		if rawJSON == "" {
			payload, err := json.Marshal(plan.Arguments)
			if err != nil {
				return nil, err
			}
			rawJSON = string(payload)
		}
		toolCalls = append(toolCalls, schema.ToolCall{
			ID:   "call_" + uuid.NewString(),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      plan.Name,
				Arguments: rawJSON,
			},
		})
	}
	return schema.AssistantMessage("", toolCalls), nil
}
