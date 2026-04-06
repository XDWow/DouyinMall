package node

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
)

// ToolExecNode 负责校验并执行工具调用计划。
type ToolExecNode struct {
	Registry *agenttool.Registry
}

func NewToolExecNode(registry *agenttool.Registry) *ToolExecNode {
	return &ToolExecNode{Registry: registry}
}

type ToolExecutionInput struct {
	CallMessage *schema.Message
	Plans       []domain.ToolCallPlan
	Mode        agenttool.ToolExecutionMode
}

func (n *ToolExecNode) Invoke(ctx context.Context, input ToolExecutionInput) ([]*schema.Message, error) {
	if n.Registry == nil {
		return nil, fmt.Errorf("工具注册表未配置")
	}
	if len(input.Plans) == 0 && input.CallMessage == nil {
		return nil, nil
	}

	callMessage := input.CallMessage
	if callMessage == nil {
		msg, err := createToolCallMessage(input.Plans)
		if err != nil {
			return nil, err
		}
		callMessage = msg
	}
	if callMessage == nil {
		return nil, nil
	}
	if err := n.Registry.ValidatePlans(input.Plans, input.Mode); err != nil {
		return nil, err
	}

	toolsNode, err := n.Registry.ToolsNode(input.Mode)
	if err != nil {
		return nil, err
	}
	return toolsNode.Invoke(ctx, callMessage)
}

func createToolCallMessage(plans []domain.ToolCallPlan) (*schema.Message, error) {
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
