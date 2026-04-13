package fallback

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	fallbacknode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/fallback"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
)

func Build(
	_ context.Context,
	ragNode *ragnode.RAGNode,
	baseQANode *fallbacknode.BaseQANode,
	l1Cache *globalnode.L1SemanticCacheNode,
	chatModel model.ToolCallingChatModel,
	registry *agenttool.Registry,
	skills *agentskill.Registry,
	maxAnswerTokens int,
) (compose.AnyGraph, error) {
	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, maxAnswerTokens)
	g := compose.NewGraph[GraphInput, Output]()

	if err := g.AddLambdaNode("FallbackL1TryNode", compose.InvokableLambda(fallbackInit(l1Cache)),
		compose.WithNodeName("FallbackL1TryNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("FallbackL1OutputNode", compose.InvokableLambda(buildFallbackL1Output),
		compose.WithNodeName("FallbackL1OutputNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("FallbackRAGNode", compose.InvokableLambda(fallbackRAG(ragNode)),
		compose.WithNodeName("FallbackRAGNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("FallbackModelAgentNode", compose.InvokableLambda(fallbackModelAgent(agent)),
		compose.WithNodeName("FallbackModelAgentNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("FallbackAgentOutputNode", compose.InvokableLambda(buildFallbackAgentOutput),
		compose.WithNodeName("FallbackAgentOutputNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("FallbackBaseQANode", compose.InvokableLambda(fallbackBaseQA(baseQANode)),
		compose.WithNodeName("FallbackBaseQANode")); err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "FallbackL1TryNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("FallbackL1TryNode", compose.NewGraphBranch(
		branchAfterFallbackL1,
		map[string]bool{
			"FallbackL1OutputNode": true,
			"FallbackRAGNode":      true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("FallbackL1OutputNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("FallbackRAGNode", "FallbackModelAgentNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("FallbackModelAgentNode", compose.NewGraphBranch(
		branchAfterFallbackAgent,
		map[string]bool{
			"FallbackAgentOutputNode": true,
			"FallbackBaseQANode":      true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("FallbackAgentOutputNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("FallbackBaseQANode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}
