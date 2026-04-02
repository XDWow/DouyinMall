package workflow

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/compose"

	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
)

func (b *Builder) BuildReturnExchangeApplyWorkflow(_ context.Context) (compose.AnyGraph, error) {
	toolWorkflow, err := b.BuildToolExecWorkflow(agenttool.ToolExecutionSerial)
	if err != nil {
		return nil, err
	}
	afterSale := b.Nodes.ReturnExchange()
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode("GetOrderDetailNode", compose.InvokableLambda(afterSale.BuildOrderQuery), compose.WithNodeName("GetOrderDetailNode")); err != nil {
		return nil, err
	}
	if toolWorkflow != nil {
		if err := g.AddGraphNode("CallReturnOrderServiceNode", toolWorkflow, compose.WithNodeName("CallReturnOrderServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("ReturnOrderResultNode", compose.InvokableLambda(afterSale.ApplyOrderResult), compose.WithNodeName("ReturnOrderResultNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("EligibilityCheckNode", compose.InvokableLambda(afterSale.EligibilityCheck), compose.WithNodeName("EligibilityCheckNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ConfirmSummaryNode", compose.InvokableLambda(afterSale.ConfirmSummary), compose.WithNodeName("ConfirmSummaryNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("BuildAfterSaleSubmitNode", compose.InvokableLambda(afterSale.BuildSubmitRequest), compose.WithNodeName("BuildAfterSaleSubmitNode")); err != nil {
		return nil, err
	}
	if toolWorkflow != nil {
		if err := g.AddGraphNode("CallAfterSaleServiceNode", toolWorkflow, compose.WithNodeName("CallAfterSaleServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("SubmitAfterSaleNode", compose.InvokableLambda(afterSale.SubmitAfterSale), compose.WithNodeName("SubmitAfterSaleNode")); err != nil {
		return nil, err
	}
	edges := [][2]string{
		{compose.START, "GetOrderDetailNode"},
		{"ReturnOrderResultNode", "EligibilityCheckNode"},
		{"ConfirmSummaryNode", compose.END},
		{"SubmitAfterSaleNode", compose.END},
	}
	if toolWorkflow != nil {
		edges = append(edges,
			[2]string{"GetOrderDetailNode", "CallReturnOrderServiceNode"},
			[2]string{"CallReturnOrderServiceNode", "ReturnOrderResultNode"},
			[2]string{"CallAfterSaleServiceNode", "SubmitAfterSaleNode"},
		)
	} else {
		edges = append(edges, [2]string{"GetOrderDetailNode", "ReturnOrderResultNode"})
	}
	for _, edge := range edges {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	if err := g.AddBranch("EligibilityCheckNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if flow == nil {
				return compose.END, nil
			}
			if flow.State.NeedHandoff {
				return compose.END, nil
			}
			switch strings.ToLower(strings.TrimSpace(orchestratorstate.SlotString(flow, "confirm_status"))) {
			case "confirmed":
				return "BuildAfterSaleSubmitNode", nil
			case "cancelled":
				return compose.END, nil
			}
			if flow.State.AwaitingConfirm {
				return "ConfirmSummaryNode", nil
			}
			return compose.END, nil
		},
		map[string]bool{"ConfirmSummaryNode": true, "BuildAfterSaleSubmitNode": true, compose.END: true},
	)); err != nil {
		return nil, err
	}
	submitTargets := map[string]bool{compose.END: true}
	if toolWorkflow != nil {
		submitTargets["CallAfterSaleServiceNode"] = true
	}
	if err := g.AddBranch("BuildAfterSaleSubmitNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if toolWorkflow != nil && support.HasToolPlan(flow, "create_after_sale_request") {
				return "CallAfterSaleServiceNode", nil
			}
			return compose.END, nil
		},
		submitTargets,
	)); err != nil {
		return nil, err
	}
	return g, nil
}
