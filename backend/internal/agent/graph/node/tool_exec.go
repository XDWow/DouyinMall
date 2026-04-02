package node

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/eino/schema"

	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
)

type ToolExecNode struct{ suite *Suite }

func (s *Suite) ToolExec() *ToolExecNode { return &ToolExecNode{suite: s} }

func (n *ToolExecNode) PrepareSerialMessage(ctx context.Context, flow *orchestratorstate.FlowContext) (*schema.Message, error) {
	return n.prepareMessage(ctx, flow, agenttool.ToolExecutionSerial)
}

func (n *ToolExecNode) PrepareParallelReadOnlyMessage(ctx context.Context, flow *orchestratorstate.FlowContext) (*schema.Message, error) {
	return n.prepareMessage(ctx, flow, agenttool.ToolExecutionParallelReadOnly)
}

func (n *ToolExecNode) ApplyMessages(ctx context.Context, messages []*schema.Message) (*orchestratorstate.FlowContext, error) {
	flow := orchestratorstate.ConversationFlowFromContext(ctx)
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	flow.Tool.ToolMessages = messages
	support.HydrateToolResults(flow)
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *ToolExecNode) ParseSlotInt64(flow *orchestratorstate.FlowContext, keys ...string) (int64, error) {
	raw := orchestratorstate.SlotString(flow, keys...)
	if raw == "" {
		return 0, fmt.Errorf("missing required slot")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func (n *ToolExecNode) prepareMessage(ctx context.Context, flow *orchestratorstate.FlowContext, mode agenttool.ToolExecutionMode) (*schema.Message, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	if flow.Tool.DecisionMessage == nil || len(flow.Tool.Plans) == 0 {
		return nil, fmt.Errorf("tool decision message is required")
	}
	if n.suite.deps.Registry == nil {
		return nil, fmt.Errorf("tool registry is not configured")
	}
	if err := n.suite.deps.Registry.ValidatePlans(flow.Tool.Plans, mode); err != nil {
		return nil, err
	}
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow.Tool.DecisionMessage, nil
}
