package addtocart

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
)

func Build(ctx context.Context, registry *agenttool.Registry, node *orchestratornode.AddToCartNode) (compose.AnyGraph, error) {
	toolGraph, err := toolexec.Build(ctx, registry, agenttool.ToolExecutionSerial)
	if err != nil {
		return nil, err
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("BuildAddToCartNode", compose.InvokableLambda(node.BuildRequest), compose.WithNodeName("BuildAddToCartNode")); err != nil {
		return nil, err
	}
	if toolGraph != nil {
		if err := g.AddGraphNode("CallCartServiceNode", toolGraph, compose.WithNodeName("CallCartServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("CartToolResultNode", compose.InvokableLambda(node.ApplyResult), compose.WithNodeName("CartToolResultNode")); err != nil {
		return nil, err
	}
	edges := [][2]string{{"CartToolResultNode", compose.END}}
	if toolGraph != nil {
		edges = append(edges,
			[2]string{compose.START, "BuildAddToCartNode"},
			[2]string{"BuildAddToCartNode", "CallCartServiceNode"},
			[2]string{"CallCartServiceNode", "CartToolResultNode"},
		)
	} else {
		edges = append(edges,
			[2]string{compose.START, "BuildAddToCartNode"},
			[2]string{"BuildAddToCartNode", "CartToolResultNode"},
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
