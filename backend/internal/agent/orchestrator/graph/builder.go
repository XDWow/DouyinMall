package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	aftersalenode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/aftersale"
	cartnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/cart"
	baseqanode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/fallback"
	inventorynode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/inventory"
	ordernode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/order"
	productnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/product"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	orchestratorragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart"
	baseqa "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/fallback"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnexchange"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnpolicy"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type Config struct {
	InterruptBeforeNodes []string
}

// Builder 主图依赖注入。
type Builder struct {
	Config          Config
	CheckpointStore compose.CheckPointStore
	Registry        *agenttool.Registry

	AccessGuard     *globalnode.AccessGuardNode
	SessionLoad     *globalnode.SessionLoadNode
	CachePreCheck   *globalnode.CachePreCheckNode
	L0ExactCache    *globalnode.L0ExactCacheNode
	L1SemanticCache *globalnode.L1SemanticCacheNode
	IntentAndSlot   *globalnode.IntentAndSlotNode
	// SlotMerge 逻辑在 IntentAndSlotNode 的 StatePre 内紧随意图之后执行（不再单独占主图节点）。
	SlotMerge *globalnode.SlotMergeNode
	AskUser   *globalnode.AskUserNode
	Route     *globalnode.RouteNode
	Finalize  *globalnode.FinalizeNode

	OrderRead           *ordernode.OrderReadNode
	InventoryRead       *inventorynode.InventoryReadNode
	ProductInfo         *productnode.ProductInfoNode
	AddToCart           *cartnode.AddToCartNode
	ReturnExchangeQuery *aftersalenode.ReturnExchangeQueryNode
	EligibilityCheck    *aftersalenode.EligibilityCheckNode
	ConfirmSummary      *aftersalenode.ConfirmSummaryNode
	SubmitAfterSale     *aftersalenode.SubmitAfterSaleNode
	RAG                 *orchestratorragnode.RAGNode
	BaseQA              *baseqanode.BaseQANode

	ChatModel       model.ToolCallingChatModel
	Skills          *agentskill.Registry
	MaxAnswerTokens int
}

func (b *Builder) Build(ctx context.Context) (compose.Runnable[map[string]any, *domain.State], error) {
	g := compose.NewGraph[map[string]any, *domain.State](
		compose.WithGenLocalState(func(context.Context) *domain.State {
			return &domain.State{}
		}),
	)

	if err := b.addPipelineNodes(g); err != nil {
		return nil, err
	}
	if err := b.addSubgraphs(ctx, g); err != nil {
		return nil, err
	}
	if err := b.addEdges(g); err != nil {
		return nil, err
	}
	if err := b.addBranches(g); err != nil {
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
	return g.Compile(ctx, opts...)
}

func (b *Builder) addPipelineNodes(g *compose.Graph[map[string]any, *domain.State]) error {
	if err := g.AddLambdaNode("AccessGuardNode", compose.InvokableLambda(statePassthrough()),
		compose.WithNodeName("AccessGuardNode"),
		compose.WithInputKey("flow"),
		compose.WithStatePreHandler(accessGuardStatePre(b.AccessGuard)),
	); err != nil {
		return err
	}

	if err := g.AddLambdaNode("SessionLoadNode", compose.InvokableLambda(statePassthrough()),
		compose.WithNodeName("SessionLoadNode"),
		compose.WithStatePreHandler(sessionLoadStatePre(b.SessionLoad)),
	); err != nil {
		return err
	}

	if err := g.AddLambdaNode("CachePreCheckNode", compose.InvokableLambda(statePassthrough()),
		compose.WithNodeName("CachePreCheckNode"),
		compose.WithStatePreHandler(cachePreCheckStatePre(b.CachePreCheck)),
	); err != nil {
		return err
	}

	if err := g.AddLambdaNode("L0ExactCacheNode", compose.InvokableLambda(statePassthrough()),
		compose.WithNodeName("L0ExactCacheNode"),
		compose.WithStatePreHandler(l0ExactCacheStatePre(b.L0ExactCache)),
	); err != nil {
		return err
	}

	if b.IntentAndSlot != nil {
		if err := g.AddLambdaNode("IntentAndSlotNode", compose.InvokableLambda(statePassthrough()),
			compose.WithNodeName("IntentAndSlotNode"),
			compose.WithStatePreHandler(intentAndSlotStatePre(b.IntentAndSlot, b.SlotMerge)),
		); err != nil {
			return err
		}
	}

	if err := b.addSubgraphBridgeNodes(g); err != nil {
		return err
	}

	if err := g.AddLambdaNode("RouteNode", compose.InvokableLambda(statePassthrough()),
		compose.WithNodeName("RouteNode"),
		compose.WithStatePreHandler(routeStatePre(b.Route)),
	); err != nil {
		return err
	}

	if err := g.AddLambdaNode("FinalizeNode", compose.InvokableLambda(statePassthrough()),
		compose.WithNodeName("FinalizeNode"),
		compose.WithStatePostHandler(finalizeStatePost(b.Finalize)),
	); err != nil {
		return err
	}
	if err := g.AddLambdaNode("InterruptNode", compose.InvokableLambda(
		func(ctx context.Context, state *domain.State) (*domain.State, error) {
			if state == nil || state.Interrupt == nil || len(state.Interrupt.Payload) == 0 {
				return state, nil
			}
			return state, compose.Interrupt(ctx, cloneSlots(state.Interrupt.Payload))
		}), compose.WithNodeName("InterruptNode")); err != nil {
		return err
	}
	return nil
}

func (b *Builder) addSubgraphs(ctx context.Context, g *compose.Graph[map[string]any, *domain.State]) error {
	type subgraphEntry struct {
		name  string
		build func() (compose.AnyGraph, error)
	}

	subgraphs := []subgraphEntry{
		{"OrderQueryGraph", func() (compose.AnyGraph, error) {
			return orderquery.Build(ctx, b.Registry, b.OrderRead, b.ChatModel, b.Skills, b.MaxAnswerTokens)
		}},
		{"InventoryGraph", func() (compose.AnyGraph, error) {
			return inventory.Build(ctx, b.Registry, b.InventoryRead, b.ChatModel, b.Skills, b.MaxAnswerTokens)
		}},
		{"ProductInfoGraph", func() (compose.AnyGraph, error) {
			return productinfo.Build(ctx, b.Registry, b.ProductInfo, b.RAG, b.L1SemanticCache, b.ChatModel, b.Skills, b.MaxAnswerTokens)
		}},
		{"AddToCartGraph", func() (compose.AnyGraph, error) {
			return addtocart.Build(ctx, b.Registry, b.AddToCart, b.ChatModel, b.Skills, b.MaxAnswerTokens)
		}},
		{"ReturnPolicyGraph", func() (compose.AnyGraph, error) {
			return returnpolicy.Build(ctx, b.RAG, b.L1SemanticCache, b.ChatModel, b.Registry, b.Skills, b.MaxAnswerTokens)
		}},
		{"ReturnExchangeGraph", func() (compose.AnyGraph, error) {
			return returnexchange.Build(ctx, b.Registry, b.ReturnExchangeQuery, b.EligibilityCheck, b.ConfirmSummary, b.SubmitAfterSale)
		}},
		{"BaseQAGraph", func() (compose.AnyGraph, error) {
			return baseqa.Build(ctx, b.RAG, b.BaseQA, b.L1SemanticCache, b.ChatModel, b.Registry, b.Skills, b.MaxAnswerTokens)
		}},
	}

	for _, entry := range subgraphs {
		graph, err := entry.build()
		if err != nil {
			return err
		}
		if graph == nil {
			continue
		}
		if err := g.AddGraphNode(entry.name, graph, compose.WithNodeName(entry.name)); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) addEdges(g *compose.Graph[map[string]any, *domain.State]) error {
	edges := [][2]string{
		{compose.START, "AccessGuardNode"},
		{"SessionLoadNode", "CachePreCheckNode"},
		{"CachePreCheckNode", "L0ExactCacheNode"},
		{"IntentAndSlotNode", "RouteNode"},
		{"PrepareOrderQueryInputNode", "OrderQueryGraph"},
		{"OrderQueryGraph", "ApplyOrderQueryResultNode"},
		{"ApplyOrderQueryResultNode", "FinalizeNode"},
		{"PrepareInventoryInputNode", "InventoryGraph"},
		{"InventoryGraph", "ApplyInventoryResultNode"},
		{"ApplyInventoryResultNode", "FinalizeNode"},
		{"PrepareProductInfoInputNode", "ProductInfoGraph"},
		{"ProductInfoGraph", "ApplyProductInfoResultNode"},
		{"ApplyProductInfoResultNode", "FinalizeNode"},
		{"PrepareAddToCartInputNode", "AddToCartGraph"},
		{"AddToCartGraph", "ApplyAddToCartResultNode"},
		{"ApplyAddToCartResultNode", "FinalizeNode"},
		{"PrepareReturnPolicyInputNode", "ReturnPolicyGraph"},
		{"ReturnPolicyGraph", "ApplyReturnPolicyResultNode"},
		{"ApplyReturnPolicyResultNode", "FinalizeNode"},
		{"PrepareReturnExchangeInputNode", "ReturnExchangeGraph"},
		{"ReturnExchangeGraph", "ApplyReturnExchangeResultNode"},
		{"ApplyReturnExchangeResultNode", "FinalizeNode"},
		{"PrepareBaseQAInputNode", "BaseQAGraph"},
		{"BaseQAGraph", "ApplyBaseQAResultNode"},
		{"ApplyBaseQAResultNode", "FinalizeNode"},
		{"InterruptNode", compose.END},
	}
	for _, edge := range edges {
		if err := g.AddEdge(edge[0], edge[1]); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) addBranches(g *compose.Graph[map[string]any, *domain.State]) error {
	if err := g.AddBranch("AccessGuardNode", compose.NewGraphBranch(
		BranchFromAccessGuard,
		map[string]bool{"FinalizeNode": true, "SessionLoadNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("L0ExactCacheNode", compose.NewGraphBranch(
		BranchAfterL0Cache,
		map[string]bool{"FinalizeNode": true, "IntentAndSlotNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("RouteNode", compose.NewGraphBranch(
		BranchFromRoute,
		map[string]bool{
			"PrepareOrderQueryInputNode":     true,
			"PrepareInventoryInputNode":      true,
			"PrepareProductInfoInputNode":    true,
			"PrepareAddToCartInputNode":      true,
			"PrepareReturnPolicyInputNode":   true,
			"PrepareReturnExchangeInputNode": true,
			"PrepareBaseQAInputNode":         true,
		},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("FinalizeNode", compose.NewGraphBranch(
		BranchAfterFinalize,
		map[string]bool{"InterruptNode": true, compose.END: true},
	)); err != nil {
		return err
	}

	return nil
}

func applySubgraphSlotWait(ctx context.Context, b *Builder, missing []string, slotQuestion string) (*domain.State, error) {
	if b == nil || b.AskUser == nil {
		return nil, fmt.Errorf("ask user node is required for slot wait")
	}
	var current *domain.State
	if err := domain.ProcessState(ctx, func(state *domain.State) error {
		if state == nil {
			return fmt.Errorf("state is nil")
		}
		current = state
		ask, err := b.AskUser.Invoke(ctx, globalnode.AskUserInput{
			Reply:            strings.TrimSpace(slotQuestion),
			Intent:           state.Session.Intent,
			IntentConfidence: state.Session.IntentConfidence,
			MissingSlots:     missing,
		})
		if err != nil {
			return err
		}
		resp := state.EnsureResponse()
		resp.Reply = ask.Reply
		resp.Intent = ask.Intent
		resp.Status = domain.ReplyStatusFallback
		resp.Confidence = ask.Confidence
		state.Session.FinalAnswer = ask.Reply
		state.Session.MissingSlots = ask.MissingSlots
		state.Session.AwaitingUser = true
		state.Answer.CacheableHint = boolPtr(false)
		state.Interrupt = &domain.InterruptState{
			Payload: map[string]any{
				"missing_slots": ask.MissingSlots,
				"question":      ask.Reply,
			},
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return current, nil
}

func applyToolFlowResult(
	ctx context.Context,
	finalAnswer string,
	needHandoff bool,
	handoffReason string,
	readOnly bool,
	_ []*schema.Message,
	cacheableHint bool,
) (*domain.State, error) {
	var current *domain.State
	if err := domain.ProcessState(ctx, func(state *domain.State) error {
		if state == nil {
			return fmt.Errorf("state is nil")
		}
		current = state
		state.Session.FinalAnswer = finalAnswer
		state.Session.AwaitingUser = false
		state.Session.MissingSlots = nil
		state.Session.NeedHandoff = needHandoff
		state.Session.HandoffReason = handoffReason
		state.Session.ReadOnly = readOnly
		state.Answer.CacheableHint = boolPtr(cacheableHint)
		support.HydrateToolResults(state)
		support.ResetToolState(state)
		return nil
	}); err != nil {
		return nil, err
	}
	return current, nil
}

func cloneSlots(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func boolPtr(value bool) *bool {
	return &value
}

func clonePendingSelectionsState(input map[string]domain.PendingSelection) map[string]domain.PendingSelection {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]domain.PendingSelection, len(input))
	for key, selection := range input {
		cloned := domain.PendingSelection{Kind: selection.Kind}
		if len(selection.Options) > 0 {
			cloned.Options = make(map[string]string, len(selection.Options))
			for optionKey, optionValue := range selection.Options {
				cloned.Options[optionKey] = optionValue
			}
		}
		out[key] = cloned
	}
	return out
}
