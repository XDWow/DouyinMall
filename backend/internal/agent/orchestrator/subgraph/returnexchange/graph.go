package returnexchange

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

func Build(
	ctx context.Context,
	registry *agenttool.Registry,
	queryNode *orchestratornode.ReturnExchangeQueryNode,
	eligibilityNode *orchestratornode.EligibilityCheckNode,
	confirmNode *orchestratornode.ConfirmSummaryNode,
	submitNode *orchestratornode.SubmitAfterSaleNode,
) (compose.AnyGraph, error) {
	toolGraph, err := toolexec.Build(ctx, registry, agenttool.ToolExecutionSerial)
	if err != nil {
		return nil, err
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("GetOrderDetailNode", compose.InvokableLambda(queryNode.BuildOrderQuery), compose.WithNodeName("GetOrderDetailNode")); err != nil {
		return nil, err
	}
	if toolGraph != nil {
		if err := g.AddGraphNode("CallReturnOrderServiceNode", toolGraph, compose.WithNodeName("CallReturnOrderServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("ReturnOrderResultNode", compose.InvokableLambda(queryNode.ApplyOrderResult), compose.WithNodeName("ReturnOrderResultNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("EligibilityCheckNode", compose.InvokableLambda(eligibilityNode.Invoke), compose.WithNodeName("EligibilityCheckNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ConfirmSummaryNode", compose.InvokableLambda(confirmNode.Invoke), compose.WithNodeName("ConfirmSummaryNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("BuildAfterSaleSubmitNode", compose.InvokableLambda(submitNode.BuildRequest), compose.WithNodeName("BuildAfterSaleSubmitNode")); err != nil {
		return nil, err
	}
	if toolGraph != nil {
		if err := g.AddGraphNode("CallAfterSaleServiceNode", toolGraph, compose.WithNodeName("CallAfterSaleServiceNode")); err != nil {
			return nil, err
		}
	}
	if err := g.AddLambdaNode("SubmitAfterSaleNode", compose.InvokableLambda(submitNode.Invoke), compose.WithNodeName("SubmitAfterSaleNode")); err != nil {
		return nil, err
	}

	edges := [][2]string{
		{"ReturnOrderResultNode", "EligibilityCheckNode"},
		{"ConfirmSummaryNode", compose.END},
		{"SubmitAfterSaleNode", compose.END},
	}
	if toolGraph != nil {
		edges = append(edges,
			[2]string{compose.START, "GetOrderDetailNode"},
			[2]string{"GetOrderDetailNode", "CallReturnOrderServiceNode"},
			[2]string{"CallReturnOrderServiceNode", "ReturnOrderResultNode"},
			[2]string{"CallAfterSaleServiceNode", "SubmitAfterSaleNode"},
		)
	} else {
		edges = append(edges,
			[2]string{compose.START, "GetOrderDetailNode"},
			[2]string{"GetOrderDetailNode", "ReturnOrderResultNode"},
		)
	}
	for _, edge := range edges {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}

	if err := g.AddBranch("EligibilityCheckNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state == nil || state.Session.NeedHandoff {
				return compose.END, nil
			}
			switch strings.ToLower(strings.TrimSpace(orchestratorstate.SlotString(state, "confirm_status"))) {
			case "confirmed":
				return "BuildAfterSaleSubmitNode", nil
			case "cancelled":
				return compose.END, nil
			}
			if state.Session.AwaitingConfirm {
				return "ConfirmSummaryNode", nil
			}
			return compose.END, nil
		},
		map[string]bool{"ConfirmSummaryNode": true, "BuildAfterSaleSubmitNode": true, compose.END: true},
	)); err != nil {
		return nil, err
	}

	submitTargets := map[string]bool{compose.END: true}
	if toolGraph != nil {
		submitTargets["CallAfterSaleServiceNode"] = true
	}
	if err := g.AddBranch("BuildAfterSaleSubmitNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if toolGraph != nil && support.HasToolPlan(state, "create_after_sale_request") {
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

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	if err := g.AddEdge(start, end); err != nil {
		return fmt.Errorf("add edge %s -> %s: %w", start, end, err)
	}
	return nil
}
