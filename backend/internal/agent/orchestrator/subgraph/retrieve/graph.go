package retrieve

import (
	"context"
	"fmt"

	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"

	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func Build(_ context.Context, retriever einoretriever.Retriever, node *orchestratornode.RetrieveNode) (compose.AnyGraph, error) {
	if retriever == nil || node == nil {
		return nil, nil
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("PrepareRetrieveQueryNode", compose.InvokableLambda(node.PrepareQuery), compose.WithNodeName("PrepareRetrieveQueryNode")); err != nil {
		return nil, err
	}
	if err := g.AddRetrieverNode("RetrieverNode", retriever, compose.WithNodeName("RetrieverNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyRetrieveNode", compose.InvokableLambda(node.ApplyDocuments), compose.WithNodeName("ApplyRetrieveNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{
		{compose.START, "PrepareRetrieveQueryNode"},
		{"PrepareRetrieveQueryNode", "RetrieverNode"},
		{"RetrieverNode", "ApplyRetrieveNode"},
		{"ApplyRetrieveNode", compose.END},
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
