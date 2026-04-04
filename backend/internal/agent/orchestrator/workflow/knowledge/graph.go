package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/retrieve"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/rewrite"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

func BuildReturnPolicy(ctx context.Context, chatModel model.ToolCallingChatModel, retriever einoretriever.Retriever, prompts *orchestratorprompt.Set, nodes *orchestratornode.Suite) (compose.AnyGraph, error) {
	return buildKnowledgeWorkflow(ctx, "ReturnPolicyStartNode", chatModel, retriever, prompts, nodes)
}

func BuildFallback(ctx context.Context, chatModel model.ToolCallingChatModel, retriever einoretriever.Retriever, prompts *orchestratorprompt.Set, nodes *orchestratornode.Suite) (compose.AnyGraph, error) {
	workflow, err := buildKnowledgeWorkflow(ctx, "FallbackStartNode", chatModel, retriever, prompts, nodes)
	if err != nil {
		return nil, err
	}
	if workflow != nil {
		return workflow, nil
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("FallbackResolveNode", compose.InvokableLambda(nodes.Fallback().Invoke), compose.WithNodeName("FallbackResolveNode")); err != nil {
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

func BuildProductDoc(ctx context.Context, chatModel model.ToolCallingChatModel, retriever einoretriever.Retriever, prompts *orchestratorprompt.Set, nodes *orchestratornode.Suite) (compose.AnyGraph, error) {
	return buildKnowledgeWorkflow(ctx, "ProductDocStartNode", chatModel, retriever, prompts, nodes)
}

func buildKnowledgeWorkflow(ctx context.Context, startNodeName string, chatModel model.ToolCallingChatModel, retrieverComp einoretriever.Retriever, prompts *orchestratorprompt.Set, nodes *orchestratornode.Suite) (compose.AnyGraph, error) {
	rewriteWorkflow, err := rewrite.Build(ctx, chatModel, prompts, nodes)
	if err != nil {
		return nil, err
	}
	retrieveWorkflow, err := retrieve.Build(ctx, retrieverComp, nodes)
	if err != nil {
		return nil, err
	}

	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode(startNodeName, compose.InvokableLambda(func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
		orchestratorstate.BindConversationState(ctx, state)
		return state, nil
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
	if err := g.AddLambdaNode("OptionalRerankNode", compose.InvokableLambda(nodes.Rerank().Invoke), compose.WithNodeName("OptionalRerankNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{
		{compose.START, startNodeName},
		{startNodeName, "QueryRewriteNode"},
		{"KnowledgeRetrieverNode", "OptionalRerankNode"},
		{"OptionalRerankNode", compose.END},
	} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	if err := g.AddBranch("QueryRewriteNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state != nil && retrieverComp != nil && strings.TrimSpace(support.FirstNonEmpty(state.Rewrite.Query, state.Session.RawQuery, state.Request.Message)) != "" {
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

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	if err := g.AddEdge(start, end); err != nil {
		return fmt.Errorf("add edge %s -> %s: %w", start, end, err)
	}
	return nil
}

