package orderquery

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
)

func Build(ctx context.Context, registry *agenttool.Registry, nodes *orchestratornode.Suite) (compose.AnyGraph, error) {
	toolWorkflow, err := toolexec.Build(ctx, registry, nodes, agenttool.ToolExecutionSerial)
	if err != nil {
		return nil, err
	}
	order := nodes.OrderRead()
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("BuildOrderQueryNode", compose.InvokableLambda(order.BuildQuery), compose.WithNodeName("BuildOrderQueryNode")); err != nil {
		return nil, err
	}
	if toolWorkflow != nil {
		if err := g.AddGraphNode("CallOrderServiceNode", toolWorkflow, compose.WithNodeName("CallOrderServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("OrderToolResultNode", compose.InvokableLambda(order.ApplyResult), compose.WithNodeName("OrderToolResultNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{
		{compose.START, "BuildOrderQueryNode"},
		{"BuildOrderQueryNode", "CallOrderServiceNode"},
		{"CallOrderServiceNode", "OrderToolResultNode"},
		{"OrderToolResultNode", compose.END},
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

