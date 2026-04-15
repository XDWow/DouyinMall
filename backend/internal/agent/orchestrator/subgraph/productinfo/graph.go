package productinfo

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	productnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/product"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
)

func Build(
	_ context.Context,
	registry *agenttool.Registry,
	productNode *productnode.ProductInfoNode,
	ragNode *ragnode.RAGNode,
	l1Cache *globalnode.L1SemanticCacheNode,
	chatModel model.ToolCallingChatModel,
	skills *agentskill.Registry,
	maxAnswerTokens int,
) (compose.AnyGraph, error) {
	if productNode == nil {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, maxAnswerTokens)
	toolExecNode := sharednode.NewToolExecNode(registry)
	g := compose.NewGraph[GraphInput, Output]()

	if err := g.AddLambdaNode("ProductInfoL1TryNode", compose.InvokableLambda(productInfoL1Try(l1Cache)),
		compose.WithNodeName("ProductInfoL1TryNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ProductInfoL1OutputNode", compose.InvokableLambda(buildProductInfoL1Output),
		compose.WithNodeName("ProductInfoL1OutputNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ProductInfoPrepareSlotsNode", compose.InvokableLambda(productInfoPrepareSlots),
		compose.WithNodeName("ProductInfoPrepareSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ProductInfoMissingSlotsNode", compose.InvokableLambda(buildProductInfoMissingOutput),
		compose.WithNodeName("ProductInfoMissingSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ProductInfoRAGNode", compose.InvokableLambda(productInfoRAG(ragNode)),
		compose.WithNodeName("ProductInfoRAGNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ProductInfoModelAgentNode", compose.InvokableLambda(productInfoModelAgent(agent)),
		compose.WithNodeName("ProductInfoModelAgentNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ProductInfoAgentAnswerNode", compose.InvokableLambda(buildProductInfoAgentOutput),
		compose.WithNodeName("ProductInfoAgentAnswerNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ProductInfoRulePlanNode", compose.InvokableLambda(productInfoRulePlanAndTools(productNode, toolExecNode)),
		compose.WithNodeName("ProductInfoRulePlanNode")); err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "ProductInfoL1TryNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("ProductInfoL1TryNode", compose.NewGraphBranch(
		branchAfterProductL1,
		map[string]bool{
			"ProductInfoL1OutputNode":     true,
			"ProductInfoPrepareSlotsNode": true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ProductInfoL1OutputNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddBranch("ProductInfoPrepareSlotsNode", compose.NewGraphBranch(
		branchAfterProductSlotCheck,
		map[string]bool{
			"ProductInfoMissingSlotsNode": true,
			"ProductInfoRAGNode":          true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ProductInfoMissingSlotsNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ProductInfoRAGNode", "ProductInfoModelAgentNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("ProductInfoModelAgentNode", compose.NewGraphBranch(
		branchAfterProductAgent,
		map[string]bool{
			"ProductInfoAgentAnswerNode": true,
			"ProductInfoRulePlanNode":    true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ProductInfoAgentAnswerNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ProductInfoRulePlanNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneSlots(input map[string]any) map[string]any {
	return cloneSlotsPI(input)
}
