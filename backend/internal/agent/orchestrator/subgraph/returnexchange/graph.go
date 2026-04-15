package returnexchange

import (
	"context"

	"github.com/cloudwego/eino/compose"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	aftersalenode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/aftersale"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

func Build(
	_ context.Context,
	registry *agenttool.Registry,
	queryNode *aftersalenode.ReturnExchangeQueryNode,
	eligibilityNode *aftersalenode.EligibilityCheckNode,
	confirmNode *aftersalenode.ConfirmSummaryNode,
	submitNode *aftersalenode.SubmitAfterSaleNode,
) (compose.AnyGraph, error) {
	if queryNode == nil || eligibilityNode == nil || confirmNode == nil || submitNode == nil {
		return nil, nil
	}

	toolExecNode := sharednode.NewToolExecNode(registry)
	g := compose.NewGraph[GraphInput, Output]()

	if err := g.AddLambdaNode("ReturnExchangeInitSlotsNode", compose.InvokableLambda(reInitSlots),
		compose.WithNodeName("ReturnExchangeInitSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnExchangeMissingSlotsNode", compose.InvokableLambda(reBuildMissingOutput),
		compose.WithNodeName("ReturnExchangeMissingSlotsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnExchangeQueryNode", compose.InvokableLambda(reRunQuery(queryNode, toolExecNode)),
		compose.WithNodeName("ReturnExchangeQueryNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnExchangeEligibilityNode", compose.InvokableLambda(reRunEligibility(eligibilityNode)),
		compose.WithNodeName("ReturnExchangeEligibilityNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnExchangeConfirmNode", compose.InvokableLambda(reConfirmIfNeeded(confirmNode)),
		compose.WithNodeName("ReturnExchangeConfirmNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnExchangeSubmitToolsNode", compose.InvokableLambda(reSubmitToolsAndHydrate(registry, toolExecNode)),
		compose.WithNodeName("ReturnExchangeSubmitToolsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnExchangeSubmitInvokeNode", compose.InvokableLambda(reRunSubmit(submitNode)),
		compose.WithNodeName("ReturnExchangeSubmitInvokeNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnExchangeAssembleOutputNode", compose.InvokableLambda(reAssembleOutput),
		compose.WithNodeName("ReturnExchangeAssembleOutputNode")); err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "ReturnExchangeInitSlotsNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("ReturnExchangeInitSlotsNode", compose.NewGraphBranch(
		branchAfterRESlotCheck,
		map[string]bool{
			"ReturnExchangeMissingSlotsNode": true,
			"ReturnExchangeQueryNode":        true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnExchangeMissingSlotsNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnExchangeQueryNode", "ReturnExchangeEligibilityNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnExchangeEligibilityNode", "ReturnExchangeConfirmNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("ReturnExchangeConfirmNode", compose.NewGraphBranch(
		branchAfterREConfirmPhase,
		map[string]bool{
			"ReturnExchangeSubmitToolsNode":    true,
			"ReturnExchangeAssembleOutputNode": true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnExchangeSubmitToolsNode", "ReturnExchangeSubmitInvokeNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnExchangeSubmitInvokeNode", "ReturnExchangeAssembleOutputNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnExchangeAssembleOutputNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneSlots(input map[string]any) map[string]any {
	return cloneSlotsRE(input)
}
