package node

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type ToolExecNodeDeps struct {
	Registry *agenttool.Registry
}

type ToolExecNode struct{ deps ToolExecNodeDeps }

func NewToolExecNode(deps ToolExecNodeDeps) *ToolExecNode {
	return &ToolExecNode{deps: deps}
}

func (n *ToolExecNode) PrepareSerialMessage(ctx context.Context, state *orchestratorstate.ConversationState) (*schema.Message, error) {
	return n.prepareMessage(ctx, state, agenttool.ToolExecutionSerial)
}

func (n *ToolExecNode) PrepareParallelReadOnlyMessage(ctx context.Context, state *orchestratorstate.ConversationState) (*schema.Message, error) {
	return n.prepareMessage(ctx, state, agenttool.ToolExecutionParallelReadOnly)
}

func (n *ToolExecNode) ApplyMessages(ctx context.Context, messages []*schema.Message) (*orchestratorstate.ConversationState, error) {
	state := orchestratorstate.ConversationStateFromContext(ctx)
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	state.Tool.ToolMessages = messages
	support.HydrateToolResults(state)
	orchestratorstate.BindConversationState(ctx, state)
	return state, nil
}

func (n *ToolExecNode) prepareMessage(ctx context.Context, state *orchestratorstate.ConversationState, mode agenttool.ToolExecutionMode) (*schema.Message, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	if state.Tool.DecisionMessage == nil || len(state.Tool.Plans) == 0 {
		return nil, fmt.Errorf("tool decision message is required")
	}
	if n.deps.Registry == nil {
		return nil, fmt.Errorf("tool registry is not configured")
	}
	if err := n.deps.Registry.ValidatePlans(state.Tool.Plans, mode); err != nil {
		return nil, err
	}
	orchestratorstate.BindConversationState(ctx, state)
	return state.Tool.DecisionMessage, nil
}
