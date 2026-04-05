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

func Build(ctx context.Context, registry *agenttool.Registry, node *orchestratornode.InventoryReadNode) (compose.AnyGraph, error) {
	toolGraph, err := toolexec.Build(ctx, registry, agenttool.ToolExecutionSerial)
	if err != nil {
		return nil, err
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("BuildInventoryQueryNode", compose.InvokableLambda(node.BuildQuery), compose.WithNodeName("BuildInventoryQueryNode")); err != nil {
		return nil, err
	}
	if toolGraph != nil {
		if err := g.AddGraphNode("CallInventoryServiceNode", toolGraph, compose.WithNodeName("CallInventoryServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("InventoryToolResultNode", compose.InvokableLambda(node.ApplyResult), compose.WithNodeName("InventoryToolResultNode")); err != nil {
		return nil, err
	}
	edges := [][2]string{{"InventoryToolResultNode", compose.END}}
	if toolGraph != nil {
		edges = append(edges,
			[2]string{compose.START, "BuildInventoryQueryNode"},
			[2]string{"BuildInventoryQueryNode", "CallInventoryServiceNode"},
			[2]string{"CallInventoryServiceNode", "InventoryToolResultNode"},
		)
	} else {
		edges = append(edges,
			[2]string{compose.START, "BuildInventoryQueryNode"},
			[2]string{"BuildInventoryQueryNode", "InventoryToolResultNode"},
		)
	}
	for _, edge := range edges {
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
