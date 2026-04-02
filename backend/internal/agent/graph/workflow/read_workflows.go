package workflow

import (
	"context"

	"github.com/cloudwego/eino/compose"

	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
)

func (b *Builder) BuildOrderQueryWorkflow(_ context.Context) (compose.AnyGraph, error) {
	toolWorkflow, err := b.BuildToolExecWorkflow(agenttool.ToolExecutionSerial)
	if err != nil {
		return nil, err
	}
	order := b.Nodes.OrderRead()
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode("NormalizeOrderIntentNode", compose.InvokableLambda(order.NormalizeIntent), compose.WithNodeName("NormalizeOrderIntentNode")); err != nil {
		return nil, err
	}
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
	for _, edge := range [][2]string{{compose.START, "NormalizeOrderIntentNode"}, {"NormalizeOrderIntentNode", "BuildOrderQueryNode"}, {"BuildOrderQueryNode", "CallOrderServiceNode"}, {"CallOrderServiceNode", "OrderToolResultNode"}, {"OrderToolResultNode", compose.END}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (b *Builder) BuildInventoryWorkflow(_ context.Context) (compose.AnyGraph, error) {
	toolWorkflow, err := b.BuildToolExecWorkflow(agenttool.ToolExecutionSerial)
	if err != nil {
		return nil, err
	}
	inventory := b.Nodes.InventoryRead()
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode("BuildInventoryQueryNode", compose.InvokableLambda(inventory.BuildQuery), compose.WithNodeName("BuildInventoryQueryNode")); err != nil {
		return nil, err
	}
	if toolWorkflow != nil {
		if err := g.AddGraphNode("CallInventoryServiceNode", toolWorkflow, compose.WithNodeName("CallInventoryServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("InventoryToolResultNode", compose.InvokableLambda(inventory.ApplyResult), compose.WithNodeName("InventoryToolResultNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{{compose.START, "BuildInventoryQueryNode"}, {"BuildInventoryQueryNode", "CallInventoryServiceNode"}, {"CallInventoryServiceNode", "InventoryToolResultNode"}, {"InventoryToolResultNode", compose.END}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (b *Builder) BuildProductInfoWorkflow(_ context.Context) (compose.AnyGraph, error) {
	toolWorkflow, err := b.BuildToolExecWorkflow(agenttool.ToolExecutionParallelReadOnly)
	if err != nil {
		return nil, err
	}
	knowledgeWorkflow, err := b.BuildKnowledgeWorkflow("ProductDocStartNode")
	if err != nil {
		return nil, err
	}
	product := b.Nodes.ProductInfo()
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode("ProductInfoIntentSplitNode", compose.InvokableLambda(product.SplitIntent), compose.WithNodeName("ProductInfoIntentSplitNode")); err != nil {
		return nil, err
	}
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
	for _, edge := range [][2]string{{compose.START, "ProductInfoIntentSplitNode"}, {"ProductInfoIntentSplitNode", "BuildProductInfoNode"}, {"BuildProductInfoNode", "CallProductServiceNode"}, {"CallProductServiceNode", "ProductToolResultNode"}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	if err := g.AddBranch("ProductToolResultNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if flow != nil && support.IsAdvisoryProductInfo(flow.State.RawQuery) && b.Retriever != nil {
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
