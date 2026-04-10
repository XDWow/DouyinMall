package inventory

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	inventorynode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/inventory"
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
	node *inventorynode.InventoryReadNode,
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
	if err := g.AddLambdaNode("InventoryCheckSlotsNode", compose.InvokableLambda(inventoryCheckSlots()),
		compose.WithNodeName("InventoryCheckSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("InventoryMissingSlotsNode", compose.InvokableLambda(buildInventoryMissingOutput),
		compose.WithNodeName("InventoryMissingSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("InventoryModelAgentNode", compose.InvokableLambda(runInventoryModelAgent(agent, skills)),
		compose.WithNodeName("InventoryModelAgentNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("InventoryAgentAnswerNode", compose.InvokableLambda(buildInventoryOutputFromAgent),
		compose.WithNodeName("InventoryAgentAnswerNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("InventoryRulePlanNode", compose.InvokableLambda(runInventoryRulePlanAndTools(node, toolExecNode)),
		compose.WithNodeName("InventoryRulePlanNode")); err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "InventoryCheckSlotsNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("InventoryCheckSlotsNode", compose.NewGraphBranch(
		branchAfterInventorySlotCheck,
		map[string]bool{
			"InventoryMissingSlotsNode": true,
			"InventoryModelAgentNode":   true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("InventoryMissingSlotsNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddBranch("InventoryModelAgentNode", compose.NewGraphBranch(
		branchAfterInventoryAgent,
		map[string]bool{
			"InventoryAgentAnswerNode": true,
			"InventoryRulePlanNode":    true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("InventoryAgentAnswerNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("InventoryRulePlanNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneSlots(input map[string]any) map[string]any {
	return cloneSlotsInv(input)
}
