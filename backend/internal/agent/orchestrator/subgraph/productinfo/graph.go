package productinfo

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/retrieve"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/rewrite"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

func Build(
	ctx context.Context,
	registry *agenttool.Registry,
	chatModel model.ToolCallingChatModel,
	retriever einoretriever.Retriever,
	prompts *orchestratorprompt.Set,
	productNode *orchestratornode.ProductInfoNode,
	rewriteNode *orchestratornode.RewriteNode,
	retrieveNode *orchestratornode.RetrieveNode,
	rerankNode *orchestratornode.RerankNode,
) (compose.AnyGraph, error) {
	toolGraph, err := toolexec.Build(ctx, registry, agenttool.ToolExecutionParallelReadOnly)
	if err != nil {
		return nil, err
	}
	rewriteGraph, err := rewrite.Build(ctx, chatModel, prompts, rewriteNode)
	if err != nil {
		return nil, err
	}
	retrieveGraph, err := retrieve.Build(ctx, retriever, retrieveNode)
	if err != nil {
		return nil, err
	}

	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("BuildProductInfoNode", compose.InvokableLambda(productNode.BuildQuery), compose.WithNodeName("BuildProductInfoNode")); err != nil {
		return nil, err
	}
	if toolGraph != nil {
		if err := g.AddGraphNode("CallProductServiceNode", toolGraph, compose.WithNodeName("CallProductServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("ProductToolResultNode", compose.InvokableLambda(productNode.ApplyResult), compose.WithNodeName("ProductToolResultNode")); err != nil {
		return nil, err
	}
	// optional RAG enrichment for advisory queries
	if rewriteGraph != nil {
		if err := g.AddGraphNode("ProductRewriteNode", rewriteGraph, compose.WithNodeName("ProductRewriteNode")); err != nil {
			return nil, err
		}
	}
	if retrieveGraph != nil {
		if err := g.AddGraphNode("ProductRetrieverNode", retrieveGraph, compose.WithNodeName("ProductRetrieverNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("ProductRerankNode", compose.InvokableLambda(rerankNode.Invoke), compose.WithNodeName("ProductRerankNode")); err != nil {
		return nil, err
	}

	edges := [][2]string{}
	if toolGraph != nil {
		edges = append(edges,
			[2]string{compose.START, "BuildProductInfoNode"},
			[2]string{"BuildProductInfoNode", "CallProductServiceNode"},
			[2]string{"CallProductServiceNode", "ProductToolResultNode"},
		)
	} else {
		edges = append(edges,
			[2]string{compose.START, "BuildProductInfoNode"},
			[2]string{"BuildProductInfoNode", "ProductToolResultNode"},
		)
	}
	if rewriteGraph != nil && retrieveGraph != nil {
		edges = append(edges,
			[2]string{"ProductRewriteNode", "ProductRetrieverNode"},
			[2]string{"ProductRetrieverNode", "ProductRerankNode"},
			[2]string{"ProductRerankNode", compose.END},
		)
	} else {
		edges = append(edges, [2]string{"ProductToolResultNode", compose.END})
	}
	for _, edge := range edges {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}

	if rewriteGraph != nil && retrieveGraph != nil {
		if err := g.AddBranch("ProductToolResultNode", compose.NewGraphBranch(
			func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
				state := orchestratorstate.ConversationStateFromContext(ctx)
				if state != nil && support.IsAdvisoryProductInfo(state.Session.RawQuery) {
					return "ProductRewriteNode", nil
				}
				return compose.END, nil
			},
			map[string]bool{"ProductRewriteNode": true, compose.END: true},
		)); err != nil {
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
