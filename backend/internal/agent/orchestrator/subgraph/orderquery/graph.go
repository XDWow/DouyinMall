package orderquery

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	ordernode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/order"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

func Build(
	_ context.Context,
	registry *agenttool.Registry,
	node *ordernode.OrderReadNode,
	chatModel model.ToolCallingChatModel,
	skills *agentskill.Registry,
	maxAnswerTokens int,
) (compose.AnyGraph, error) {
	if node == nil {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, maxAnswerTokens)
	toolExec := sharednode.NewToolExecNode(registry)

	g := compose.NewGraph[GraphInput, Output]()
	if err := g.AddLambdaNode("OrderQueryPrepareSlotsNode", compose.InvokableLambda(prepareSlots()),
		compose.WithNodeName("OrderQueryPrepareSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("OrderQueryModelAgentNode", compose.InvokableLambda(runOrderModelAgent(agent, skills)),
		compose.WithNodeName("OrderQueryModelAgentNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("OrderQueryRulePlanNode", compose.InvokableLambda(runOrderRulePlanAndTools(node, toolExec)),
		compose.WithNodeName("OrderQueryRulePlanNode")); err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "OrderQueryPrepareSlotsNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("OrderQueryPrepareSlotsNode", "OrderQueryModelAgentNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("OrderQueryModelAgentNode", "OrderQueryRulePlanNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("OrderQueryRulePlanNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}
