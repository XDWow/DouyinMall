package support

import (
	"encoding/json"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type ToolCallSpec struct {
	Name      string
	Arguments map[string]any
}

func BuildToolCallMessage(name string, arguments map[string]any) (*schema.Message, error) {
	return BuildToolCallsMessage(ToolCallSpec{
		Name:      name,
		Arguments: arguments,
	})
}

func BuildToolCallsMessage(calls ...ToolCallSpec) (*schema.Message, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	toolCalls := make([]schema.ToolCall, 0, len(calls))
	for _, call := range calls {
		payload, err := json.Marshal(call.Arguments)
		if err != nil {
			return nil, err
		}
		toolCalls = append(toolCalls, schema.ToolCall{
			ID:   "call_" + uuid.NewString(),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      call.Name,
				Arguments: string(payload),
			},
		})
	}
	return schema.AssistantMessage("", toolCalls), nil
}
