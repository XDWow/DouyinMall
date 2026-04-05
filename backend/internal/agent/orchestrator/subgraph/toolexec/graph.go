package toolexec

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func Build(_ context.Context, registry *agenttool.Registry, mode agenttool.ToolExecutionMode) (compose.AnyGraph, error) {
	if registry == nil {
		return nil, nil
	}
	toolsNode, err := registry.ToolsNode(mode)
	if err != nil {
		return nil, err
	}
	execNode := orchestratornode.NewToolExecNode(orchestratornode.ToolExecNodeDeps{Registry: registry})
	prepareName := "PrepareSerialToolMessageNode"
	prepareFn := execNode.PrepareSerialMessage
	if mode == agenttool.ToolExecutionParallelReadOnly {
		prepareName = "PrepareParallelReadonlyToolMessageNode"
		prepareFn = execNode.PrepareParallelReadOnlyMessage
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode(prepareName, compose.InvokableLambda(prepareFn), compose.WithNodeName(prepareName)); err != nil {
		return nil, err
	}
	if err := g.AddToolsNode("ToolsNode", toolsNode, compose.WithNodeName("ToolsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyToolMessagesNode", compose.InvokableLambda(execNode.ApplyMessages), compose.WithNodeName("ApplyToolMessagesNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{
		{compose.START, prepareName},
		{prepareName, "ToolsNode"},
		{"ToolsNode", "ApplyToolMessagesNode"},
		{"ApplyToolMessagesNode", compose.END},
	} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	if err := g.AddEdge(start, end); err != nil {
		return fmt.Errorf("add edge %s -> %s: %w", start, end, err)
	}
	return nil
}
