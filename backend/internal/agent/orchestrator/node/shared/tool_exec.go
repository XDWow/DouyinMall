package shared

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

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
		return nil, fmt.Errorf("tool registry is not configured")
	}
	if len(input.Plans) == 0 && input.CallMessage == nil {
		return nil, nil
	}

	callMessage := input.CallMessage
	if callMessage == nil {
		msg, err := support.BuildToolCallMessage(input.Plans)
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
