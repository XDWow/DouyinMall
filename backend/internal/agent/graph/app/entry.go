package app

import (
	"context"

	"github.com/cloudwego/eino/compose"

	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/graph/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	orchestratorworkflow "github.com/XDWow/DouyinMall/backend/internal/agent/graph/workflow"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

// Config controls graph-level interrupt behavior for the main agent graph.
type Config struct {
	InterruptBeforeNodes []string
	InterruptAfterNodes  []string
}

// Builder assembles the main Eino graph. Graph is the primary orchestrator
// for this service; workflow subgraphs are only used for a few reusable
// business paths.
type Builder struct {
	Config          Config
	CheckpointStore cache.CheckpointStore
	Nodes           *orchestratornode.Suite
	Workflows       *orchestratorworkflow.Builder
}

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	return g.AddEdge(start, end)
}

func (b *Builder) Build(ctx context.Context) (compose.Runnable[map[string]any, *orchestratorstate.FlowContext], error) {
	g := compose.NewGraph[map[string]any, *orchestratorstate.FlowContext](
		compose.WithGenLocalState(func(context.Context) *orchestratorstate.ConversationState {
			return &orchestratorstate.ConversationState{}
		}),
	)

	if err := g.AddLambdaNode("AccessGuardNode", compose.InvokableLambda(b.Nodes.AccessGuard().Invoke), compose.WithNodeName("AccessGuardNode"), compose.WithInputKey("flow")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("SessionLoadNode", compose.InvokableLambda(b.Nodes.SessionLoad().Invoke), compose.WithNodeName("SessionLoadNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("L0ExactCacheNode", compose.InvokableLambda(b.Nodes.L0ExactCache().Invoke), compose.WithNodeName("L0ExactCacheNode")); err != nil {
		return nil, err
	}

	intentChain, err := b.Workflows.BuildIntentClassificationChain()
	if err != nil {
		return nil, err
	}
	if intentChain != nil {
		if err := g.AddGraphNode("IntentClassifyChain", intentChain, compose.WithNodeName("IntentClassifyChain")); err != nil {
			return nil, err
		}
	} else if err := g.AddLambdaNode("IntentClassifyChain", compose.InvokableLambda(b.Nodes.IntentClassify().Invoke), compose.WithNodeName("IntentClassifyChain")); err != nil {
		return nil, err
	}

	if err := g.AddLambdaNode("SlotExtractNode", compose.InvokableLambda(b.Nodes.SlotExtract().Invoke), compose.WithNodeName("SlotExtractNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("SlotCheckNode", compose.InvokableLambda(b.Nodes.SlotCheck().Invoke), compose.WithNodeName("SlotCheckNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("AskUserNode", compose.InvokableLambda(b.Nodes.AskUser().Invoke), compose.WithNodeName("AskUserNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("RouteNode", compose.InvokableLambda(b.Nodes.Route().Invoke), compose.WithNodeName("RouteNode")); err != nil {
		return nil, err
	}

	builders := []struct {
		name    string
		builder func(context.Context) (compose.AnyGraph, error)
	}{
		{name: "OrderQueryWorkflow", builder: b.Workflows.BuildOrderQueryWorkflow},
		{name: "ReturnPolicyRAGWorkflow", builder: b.Workflows.BuildReturnPolicyRAGWorkflow},
		{name: "InventoryWorkflow", builder: b.Workflows.BuildInventoryWorkflow},
		{name: "ProductInfoWorkflow", builder: b.Workflows.BuildProductInfoWorkflow},
		{name: "ReturnExchangeApplyWorkflow", builder: b.Workflows.BuildReturnExchangeApplyWorkflow},
		{name: "FallbackWorkflow", builder: b.Workflows.BuildFallbackWorkflow},
	}
	for _, item := range builders {
		wf, buildErr := item.builder(ctx)
		if buildErr != nil {
			return nil, buildErr
		}
		if err := g.AddGraphNode(item.name, wf, compose.WithNodeName(item.name)); err != nil {
			return nil, err
		}
	}

	if err := g.AddLambdaNode("ResponseRenderNode", compose.InvokableLambda(b.Nodes.ResponseRender().Invoke), compose.WithNodeName("ResponseRenderNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("CacheWritebackNode", compose.InvokableLambda(b.Nodes.CacheWriteback().Invoke), compose.WithNodeName("CacheWritebackNode")); err != nil {
		return nil, err
	}

	for _, edge := range [][2]string{
		{compose.START, "AccessGuardNode"},
		{"AccessGuardNode", "SessionLoadNode"},
		{"SessionLoadNode", "L0ExactCacheNode"},
		{"IntentClassifyChain", "SlotExtractNode"},
		{"SlotExtractNode", "SlotCheckNode"},
		{"AskUserNode", compose.END},
		{"OrderQueryWorkflow", "ResponseRenderNode"},
		{"ReturnPolicyRAGWorkflow", "ResponseRenderNode"},
		{"InventoryWorkflow", "ResponseRenderNode"},
		{"ProductInfoWorkflow", "ResponseRenderNode"},
		{"ReturnExchangeApplyWorkflow", "ResponseRenderNode"},
		{"FallbackWorkflow", "ResponseRenderNode"},
		{"ResponseRenderNode", "CacheWritebackNode"},
		{"CacheWritebackNode", compose.END},
	} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}

	if err := g.AddBranch("L0ExactCacheNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if flow != nil && flow.State.CacheHitLevel == "L0" {
				return "ResponseRenderNode", nil
			}
			return "IntentClassifyChain", nil
		},
		map[string]bool{"ResponseRenderNode": true, "IntentClassifyChain": true},
	)); err != nil {
		return nil, err
	}

	if err := g.AddBranch("SlotCheckNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if flow != nil && flow.State.AwaitingUser {
				return "AskUserNode", nil
			}
			return "RouteNode", nil
		},
		map[string]bool{"AskUserNode": true, "RouteNode": true},
	)); err != nil {
		return nil, err
	}

	if err := g.AddBranch("RouteNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if flow == nil {
				return "FallbackWorkflow", nil
			}
			switch flow.State.Route {
			case orchestratorstate.RouteOrderQuery:
				return "OrderQueryWorkflow", nil
			case orchestratorstate.RouteReturnPolicy:
				return "ReturnPolicyRAGWorkflow", nil
			case orchestratorstate.RouteInventory:
				return "InventoryWorkflow", nil
			case orchestratorstate.RouteProductInfo:
				return "ProductInfoWorkflow", nil
			case orchestratorstate.RouteReturnExchangeApply:
				return "ReturnExchangeApplyWorkflow", nil
			default:
				return "FallbackWorkflow", nil
			}
		},
		map[string]bool{
			"OrderQueryWorkflow":          true,
			"ReturnPolicyRAGWorkflow":     true,
			"InventoryWorkflow":           true,
			"ProductInfoWorkflow":         true,
			"ReturnExchangeApplyWorkflow": true,
			"FallbackWorkflow":            true,
		},
	)); err != nil {
		return nil, err
	}

	opts := []compose.GraphCompileOption{
		compose.WithGraphName("agent_graph"),
		compose.WithNodeTriggerMode(compose.AllPredecessor),
		compose.WithEagerExecution(),
	}
	if b.CheckpointStore != nil {
		opts = append(opts, compose.WithCheckPointStore(b.CheckpointStore))
	}
	if len(b.Config.InterruptBeforeNodes) > 0 {
		opts = append(opts, compose.WithInterruptBeforeNodes(b.Config.InterruptBeforeNodes))
	}
	if len(b.Config.InterruptAfterNodes) > 0 {
		opts = append(opts, compose.WithInterruptAfterNodes(b.Config.InterruptAfterNodes))
	}

	return g.Compile(ctx, opts...)
}
