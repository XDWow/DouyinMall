package addtocart

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	cartnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/cart"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

type Output struct {
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
	AwaitingUser  bool
	MissingSlots  []string
}

func Build(
	_ context.Context,
	registry *agenttool.Registry,
	node *cartnode.AddToCartNode,
	chatModel model.ToolCallingChatModel,
	skills *agentskill.Registry,
	maxAnswerTokens int,
) (compose.AnyGraph, error) {
	if node == nil {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, maxAnswerTokens)
	toolExecNode := sharednode.NewToolExecNode(registry)

	g := compose.NewGraph[struct{}, Output]()
	if err := g.AddLambdaNode("AddToCartCheckSlotsNode", compose.InvokableLambda(addToCartCheckSlots()),
		compose.WithNodeName("AddToCartCheckSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("AddToCartMissingSlotsNode", compose.InvokableLambda(buildCartMissingOutput),
		compose.WithNodeName("AddToCartMissingSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("AddToCartModelAgentNode", compose.InvokableLambda(runAddToCartModelAgent(agent, skills)),
		compose.WithNodeName("AddToCartModelAgentNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("AddToCartAgentAnswerNode", compose.InvokableLambda(buildCartOutputFromAgent),
		compose.WithNodeName("AddToCartAgentAnswerNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("AddToCartRulePlanNode", compose.InvokableLambda(runCartRulePlanAndTools(node, toolExecNode)),
		compose.WithNodeName("AddToCartRulePlanNode")); err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "AddToCartCheckSlotsNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("AddToCartCheckSlotsNode", compose.NewGraphBranch(
		branchAfterCartSlotCheck,
		map[string]bool{
			"AddToCartMissingSlotsNode": true,
			"AddToCartModelAgentNode":   true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("AddToCartMissingSlotsNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddBranch("AddToCartModelAgentNode", compose.NewGraphBranch(
		branchAfterCartAgent,
		map[string]bool{
			"AddToCartAgentAnswerNode": true,
			"AddToCartRulePlanNode":    true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("AddToCartAgentAnswerNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("AddToCartRulePlanNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneSlots(input map[string]any) map[string]any {
	return cloneSlotsCart(input)
}
