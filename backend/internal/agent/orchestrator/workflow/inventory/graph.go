package inventory

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
	inventoryNode := nodes.InventoryRead()
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("BuildInventoryQueryNode", compose.InvokableLambda(inventoryNode.BuildQuery), compose.WithNodeName("BuildInventoryQueryNode")); err != nil {
		return nil, err
	}
	if toolWorkflow != nil {
		if err := g.AddGraphNode("CallInventoryServiceNode", toolWorkflow, compose.WithNodeName("CallInventoryServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("InventoryToolResultNode", compose.InvokableLambda(inventoryNode.ApplyResult), compose.WithNodeName("InventoryToolResultNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{
		{compose.START, "BuildInventoryQueryNode"},
		{"BuildInventoryQueryNode", "CallInventoryServiceNode"},
		{"CallInventoryServiceNode", "InventoryToolResultNode"},
		{"InventoryToolResultNode", compose.END},
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

