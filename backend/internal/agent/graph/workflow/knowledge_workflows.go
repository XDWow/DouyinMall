package workflow

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/compose"

	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
)

func (b *Builder) BuildReturnPolicyRAGWorkflow(_ context.Context) (compose.AnyGraph, error) {
	return b.BuildKnowledgeWorkflow("ReturnPolicyStartNode")
}

func (b *Builder) BuildFallbackWorkflow(_ context.Context) (compose.AnyGraph, error) {
	workflow, err := b.BuildKnowledgeWorkflow("FallbackStartNode")
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
		if err := g.AddLambdaNode("FallbackResolveNode", compose.InvokableLambda(b.Nodes.Fallback().Invoke), compose.WithNodeName("FallbackResolveNode")); err != nil {
			return nil, err
		}
		if err := addEdge(g, compose.START, "FallbackResolveNode"); err != nil {
			return nil, err
		}
		if err := addEdge(g, "FallbackResolveNode", compose.END); err != nil {
			return nil, err
		}
		return g, nil
	}
	return workflow, nil
}

func (b *Builder) BuildKnowledgeWorkflow(startNodeName string) (compose.AnyGraph, error) {
	rewriteWorkflow, err := b.BuildRewriteChain()
	if err != nil {
		return nil, err
	}
	retrieveWorkflow, err := b.BuildRetrieveChain()
	if err != nil {
		return nil, err
	}

	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	return b.buildKnowledgeWorkflowWithGraphs(startNodeName, g, rewriteWorkflow, retrieveWorkflow)
}

func (b *Builder) buildKnowledgeWorkflowWithGraphs(startNodeName string, g *compose.Graph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext], rewriteWorkflow, retrieveWorkflow compose.AnyGraph) (compose.AnyGraph, error) {
	if err := g.AddLambdaNode(startNodeName, compose.InvokableLambda(func(ctx context.Context, flow *orchestratorstate.FlowContext) (*orchestratorstate.FlowContext, error) {
		orchestratorstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}), compose.WithNodeName(startNodeName)); err != nil {
		return nil, err
	}
	if rewriteWorkflow != nil {
		if err := g.AddGraphNode("QueryRewriteNode", rewriteWorkflow, compose.WithNodeName("QueryRewriteNode")); err != nil {
			return nil, err
		}
	}
	if retrieveWorkflow != nil {
		if err := g.AddGraphNode("KnowledgeRetrieverNode", retrieveWorkflow, compose.WithNodeName("KnowledgeRetrieverNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("OptionalRerankNode", compose.InvokableLambda(b.Nodes.Rerank().Invoke), compose.WithNodeName("OptionalRerankNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{{compose.START, startNodeName}, {startNodeName, "QueryRewriteNode"}, {"KnowledgeRetrieverNode", "OptionalRerankNode"}, {"OptionalRerankNode", compose.END}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	if err := g.AddBranch("QueryRewriteNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if flow != nil && b.Retriever != nil && strings.TrimSpace(support.FirstNonEmpty(flow.Rewrite.Query, flow.State.RawQuery, flow.Request.Message)) != "" {
				return "KnowledgeRetrieverNode", nil
			}
			return compose.END, nil
		},
		map[string]bool{"KnowledgeRetrieverNode": true, compose.END: true},
	)); err != nil {
		return nil, err
	}
	return g, nil
}
