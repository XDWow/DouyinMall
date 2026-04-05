package graph

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/components/tools"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/fallback"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/intentclassify"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnexchange"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnpolicy"
)

// Config controls graph-level interrupt behavior.
type Config struct {
	InterruptBeforeNodes []string
	InterruptAfterNodes  []string
}

// Builder assembles the main Eino graph from pre-built nodes and subgraph deps.
type Builder struct {
	Config          Config
	CheckpointStore cache.CheckpointStore

	// subgraph deps
	Model     model.ToolCallingChatModel
	Retriever einoretriever.Retriever
	Registry  *agenttool.Registry
	Prompts   *orchestratorprompt.Set

	// pipeline nodes (used directly in the main graph)
	AccessGuard    *orchestratornode.AccessGuardNode
	SessionLoad    *orchestratornode.SessionLoadNode
	L0ExactCache   *orchestratornode.L0ExactCacheNode
	IntentClassify *orchestratornode.IntentClassifyNode
	SlotExtract    *orchestratornode.SlotExtractNode
	SlotCheck      *orchestratornode.SlotCheckNode
	AskUser        *orchestratornode.AskUserNode
	Route          *orchestratornode.RouteNode
	ResponseRender *orchestratornode.ResponseRenderNode
	CacheWriteback *orchestratornode.CacheWritebackNode

	// business nodes (passed into subgraph builders)
	OrderRead           *orchestratornode.OrderReadNode
	InventoryRead       *orchestratornode.InventoryReadNode
	ProductInfo         *orchestratornode.ProductInfoNode
	AddToCart           *orchestratornode.AddToCartNode
	ReturnExchangeQuery *orchestratornode.ReturnExchangeQueryNode
	EligibilityCheck    *orchestratornode.EligibilityCheckNode
	ConfirmSummary      *orchestratornode.ConfirmSummaryNode
	SubmitAfterSale     *orchestratornode.SubmitAfterSaleNode
	Rewrite             *orchestratornode.RewriteNode
	Retrieve            *orchestratornode.RetrieveNode
	Rerank              *orchestratornode.RerankNode
	Fallback            *orchestratornode.FallbackNode
}

func (b *Builder) Build(ctx context.Context) (compose.Runnable[map[string]any, *orchestratorstate.ConversationState], error) {
	g := compose.NewGraph[map[string]any, *orchestratorstate.ConversationState](
		compose.WithGenLocalState(func(context.Context) *orchestratorstate.ConversationState {
			return &orchestratorstate.ConversationState{}
		}),
	)

	if err := g.AddLambdaNode("AccessGuardNode", compose.InvokableLambda(b.AccessGuard.Invoke), compose.WithNodeName("AccessGuardNode"), compose.WithInputKey("flow")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("SessionLoadNode", compose.InvokableLambda(b.SessionLoad.Invoke), compose.WithNodeName("SessionLoadNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("L0ExactCacheNode", compose.InvokableLambda(b.L0ExactCache.Invoke), compose.WithNodeName("L0ExactCacheNode")); err != nil {
		return nil, err
	}

	intentGraph, err := intentclassify.Build(ctx, b.Model, b.Prompts, b.IntentClassify)
	if err != nil {
		return nil, err
	}
	if intentGraph != nil {
		if err := g.AddGraphNode("IntentClassifyChain", intentGraph, compose.WithNodeName("IntentClassifyChain")); err != nil {
			return nil, err
		}
	} else {
		if err := g.AddLambdaNode("IntentClassifyChain", compose.InvokableLambda(b.IntentClassify.Invoke), compose.WithNodeName("IntentClassifyChain")); err != nil {
			return nil, err
		}
	}

	if err := g.AddLambdaNode("SlotExtractNode", compose.InvokableLambda(b.SlotExtract.Invoke), compose.WithNodeName("SlotExtractNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("SlotCheckNode", compose.InvokableLambda(b.SlotCheck.Invoke), compose.WithNodeName("SlotCheckNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("AskUserNode", compose.InvokableLambda(b.AskUser.Invoke), compose.WithNodeName("AskUserNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("RouteNode", compose.InvokableLambda(b.Route.Invoke), compose.WithNodeName("RouteNode")); err != nil {
		return nil, err
	}

	type subgraphEntry struct {
		name  string
		build func() (compose.AnyGraph, error)
	}
	subgraphs := []subgraphEntry{
		{"OrderQueryGraph", func() (compose.AnyGraph, error) {
			return orderquery.Build(ctx, b.Registry, b.OrderRead)
		}},
		{"InventoryGraph", func() (compose.AnyGraph, error) {
			return inventory.Build(ctx, b.Registry, b.InventoryRead)
		}},
		{"ProductInfoGraph", func() (compose.AnyGraph, error) {
			return productinfo.Build(ctx, b.Registry, b.Model, b.Retriever, b.Prompts, b.ProductInfo, b.Rewrite, b.Retrieve, b.Rerank)
		}},
		{"AddToCartGraph", func() (compose.AnyGraph, error) {
			return addtocart.Build(ctx, b.Registry, b.AddToCart)
		}},
		{"ReturnPolicyGraph", func() (compose.AnyGraph, error) {
			return returnpolicy.Build(ctx, b.Model, b.Retriever, b.Prompts, b.Rewrite, b.Retrieve, b.Rerank)
		}},
		{"ReturnExchangeGraph", func() (compose.AnyGraph, error) {
			return returnexchange.Build(ctx, b.Registry, b.ReturnExchangeQuery, b.EligibilityCheck, b.ConfirmSummary, b.SubmitAfterSale)
		}},
		{"FallbackGraph", func() (compose.AnyGraph, error) {
			return fallback.Build(ctx, b.Model, b.Retriever, b.Prompts, b.Rewrite, b.Retrieve, b.Rerank, b.Fallback)
		}},
	}
	for _, sg := range subgraphs {
		graph, err := sg.build()
		if err != nil {
			return nil, err
		}
		if err := g.AddGraphNode(sg.name, graph, compose.WithNodeName(sg.name)); err != nil {
			return nil, err
		}
	}

	if err := g.AddLambdaNode("ResponseRenderNode", compose.InvokableLambda(b.ResponseRender.Invoke), compose.WithNodeName("ResponseRenderNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("CacheWritebackNode", compose.InvokableLambda(b.CacheWriteback.Invoke), compose.WithNodeName("CacheWritebackNode")); err != nil {
		return nil, err
	}

	for _, edge := range [][2]string{
		{compose.START, "AccessGuardNode"},
		{"AccessGuardNode", "SessionLoadNode"},
		{"SessionLoadNode", "L0ExactCacheNode"},
		{"IntentClassifyChain", "SlotExtractNode"},
		{"SlotExtractNode", "SlotCheckNode"},
		{"AskUserNode", compose.END},
		{"OrderQueryGraph", "ResponseRenderNode"},
		{"InventoryGraph", "ResponseRenderNode"},
		{"ProductInfoGraph", "ResponseRenderNode"},
		{"AddToCartGraph", "ResponseRenderNode"},
		{"ReturnPolicyGraph", "ResponseRenderNode"},
		{"ReturnExchangeGraph", "ResponseRenderNode"},
		{"FallbackGraph", "ResponseRenderNode"},
		{"ResponseRenderNode", "CacheWritebackNode"},
		{"CacheWritebackNode", compose.END},
	} {
		if err := g.AddEdge(edge[0], edge[1]); err != nil {
			return nil, err
		}
	}

	if err := g.AddBranch("L0ExactCacheNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state != nil && state.Session.CacheHitLevel == "L0" {
				return "ResponseRenderNode", nil
			}
			return "IntentClassifyChain", nil
		},
		map[string]bool{"ResponseRenderNode": true, "IntentClassifyChain": true},
	)); err != nil {
		return nil, err
	}

	if err := g.AddBranch("SlotCheckNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state != nil && state.Session.AwaitingUser {
				return "AskUserNode", nil
			}
			return "RouteNode", nil
		},
		map[string]bool{"AskUserNode": true, "RouteNode": true},
	)); err != nil {
		return nil, err
	}

	if err := g.AddBranch("RouteNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state == nil {
				return "FallbackGraph", nil
			}
			switch state.Session.Route {
			case orchestratorstate.RouteOrderQuery:
				return "OrderQueryGraph", nil
			case orchestratorstate.RouteInventory:
				return "InventoryGraph", nil
			case orchestratorstate.RouteProductInfo:
				return "ProductInfoGraph", nil
			case orchestratorstate.RouteAddToCart:
				return "AddToCartGraph", nil
			case orchestratorstate.RouteReturnPolicy:
				return "ReturnPolicyGraph", nil
			case orchestratorstate.RouteReturnExchangeApply:
				return "ReturnExchangeGraph", nil
			default:
				return "FallbackGraph", nil
			}
		},
		map[string]bool{
			"OrderQueryGraph":     true,
			"InventoryGraph":      true,
			"ProductInfoGraph":    true,
			"AddToCartGraph":      true,
			"ReturnPolicyGraph":   true,
			"ReturnExchangeGraph": true,
			"FallbackGraph":       true,
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
