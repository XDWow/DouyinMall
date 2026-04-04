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
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/workflow/knowledge"
)

func Build(ctx context.Context, chatModel model.ToolCallingChatModel, retriever einoretriever.Retriever, registry *agenttool.Registry, prompts *orchestratorprompt.Set, nodes *orchestratornode.Suite) (compose.AnyGraph, error) {
	toolWorkflow, err := toolexec.Build(ctx, registry, nodes, agenttool.ToolExecutionParallelReadOnly)
	if err != nil {
		return nil, err
	}
	knowledgeWorkflow, err := knowledge.BuildProductDoc(ctx, chatModel, retriever, prompts, nodes)
	if err != nil {
		return nil, err
	}
	product := nodes.ProductInfo()
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("BuildProductInfoNode", compose.InvokableLambda(product.BuildQuery), compose.WithNodeName("BuildProductInfoNode")); err != nil {
		return nil, err
	}
	if toolWorkflow != nil {
		if err := g.AddGraphNode("CallProductServiceNode", toolWorkflow, compose.WithNodeName("CallProductServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("ProductToolResultNode", compose.InvokableLambda(product.ApplyResult), compose.WithNodeName("ProductToolResultNode")); err != nil {
		return nil, err
	}
	if knowledgeWorkflow != nil {
		if err := g.AddGraphNode("ProductDocWorkflowNode", knowledgeWorkflow, compose.WithNodeName("ProductDocWorkflowNode")); err != nil {
			return nil, err
		}
	}
	for _, edge := range [][2]string{
		{compose.START, "BuildProductInfoNode"},
		{"BuildProductInfoNode", "CallProductServiceNode"},
		{"CallProductServiceNode", "ProductToolResultNode"},
	} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	if err := g.AddBranch("ProductToolResultNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state != nil && support.IsAdvisoryProductInfo(state.Session.RawQuery) && retriever != nil {
				return "ProductDocWorkflowNode", nil
			}
			return compose.END, nil
		},
		map[string]bool{"ProductDocWorkflowNode": true, compose.END: true},
	)); err != nil {
		return nil, err
	}
	if err := addEdge(g, "ProductDocWorkflowNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	if err := g.AddEdge(start, end); err != nil {
		return fmt.Errorf("add edge %s -> %s: %w", start, end, err)
	}
	return nil
}

