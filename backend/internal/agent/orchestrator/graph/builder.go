package graph

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	aftersalenode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/aftersale"
	cartnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/cart"
	baseqanode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/fallback"
	inventorynode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/inventory"
	ordernode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/order"
	productnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/product"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	orchestratorragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart"
	baseqa "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/fallback"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnexchange"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type Config struct {
	InterruptBeforeNodes []string
	InterruptAfterNodes  []string
}

type Builder struct {
	Config          Config
	CheckpointStore cache.CheckpointStore

	Registry *agenttool.Registry

	AccessGuard       *globalnode.AccessGuardNode
	SessionLoad       *globalnode.SessionLoadNode
	CachePreCheck     *globalnode.CachePreCheckNode
	L0ExactCache      *globalnode.L0ExactCacheNode
	L1SemanticCache   *globalnode.L1SemanticCacheNode
	QueryRewrite      *globalnode.QueryRewriteNode
	IntentClassify    *globalnode.IntentClassifyNode
	GlobalSlotExtract *globalnode.GlobalSlotExtractNode
	GlobalSlotCheck   *globalnode.GlobalSlotCheckNode
	AskUser           *globalnode.AskUserNode
	Route             *globalnode.RouteNode
	SkillSelect       *globalnode.SkillSelectNode
	Finalize          *globalnode.FinalizeNode

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
}

func (b *Builder) Build(ctx context.Context) (compose.Runnable[map[string]any, *orchestratorstate.State], error) {
	g := compose.NewGraph[map[string]any, *orchestratorstate.State](
		compose.WithGenLocalState(func(context.Context) *orchestratorstate.State {
			return &orchestratorstate.State{}
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
	if len(b.Config.InterruptAfterNodes) > 0 {
		opts = append(opts, compose.WithInterruptAfterNodes(b.Config.InterruptAfterNodes))
	}
	return g.Compile(ctx, opts...)
}

func (b *Builder) addPipelineNodes(g *compose.Graph[map[string]any, *orchestratorstate.State]) error {
	if err := g.AddLambdaNode("AccessGuardNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			result, err := b.AccessGuard.Invoke(ctx, globalnode.AccessGuardInput{
				Message:     state.Request.Message,
				UserID:      state.Request.UserID,
				ResumeToken: state.Request.ResumeToken,
			})
			if err != nil {
				return nil, err
			}
			session := &state.Session
			session.UserID = state.Request.UserID
			session.RawQuery = result.RawQuery
			session.TenantID = result.TenantID
			session.ResumeFromCP = result.ResumeFromCP
			session.ErrorCode = result.ErrorCode
			session.FinalAnswer = result.FinalAnswer
			if result.ErrorCode != "" {
				state.Answer.CacheableHint = boolPtr(false)
			}
			state.Interrupt = nil
			return state, nil
		}), compose.WithNodeName("AccessGuardNode"), compose.WithInputKey("flow")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("SessionLoadNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			result, err := b.SessionLoad.Invoke(ctx, globalnode.SessionLoadInput{
				Request:       state.Request,
				TraceID:       state.TraceID,
				ExistingSlots: cloneSlots(state.Session.Slots),
			})
			if err != nil {
				return nil, err
			}
			session := &state.Session
			state.SessionMeta = result.SessionMeta
			state.Request.SessionID = result.SessionID
			session.SessionID = result.SessionID
			state.Session.Messages = append([]*schema.Message(nil), result.RecentMessages...)
			session.Slots = result.Slots
			session.CurrentRefs = result.CurrentRefs
			session.PendingSelections = clonePendingSelectionsState(result.PendingSelections)
			state.EnsureResponse().SessionID = result.SessionID
			return state, nil
		}), compose.WithNodeName("SessionLoadNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("CachePreCheckNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			result, err := b.CachePreCheck.Invoke(ctx, globalnode.CachePreCheckInput{
				TenantID:        state.Session.TenantID,
				UserID:          state.Request.UserID,
				Message:         state.Session.RawQuery,
				ResumeFromCP:    state.Session.ResumeFromCP,
				AwaitingUser:    state.Session.AwaitingUser,
				AwaitingConfirm: state.Session.AwaitingConfirm,
			})
			if err != nil {
				return nil, err
			}
			state.Cache.AllowExact = result.AllowExact
			state.Cache.AllowSemantic = result.AllowSemantic
			state.Cache.IntentBucket = result.IntentBucket
			state.Cache.Scope = string(result.Scope)
			return state, nil
		}), compose.WithNodeName("CachePreCheckNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("L0ExactCacheNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			result, err := b.L0ExactCache.Invoke(ctx, globalnode.L0ExactCacheInput{
				TenantID:     state.Session.TenantID,
				UserID:       state.Request.UserID,
				RawQuery:     state.Session.RawQuery,
				SessionID:    state.Request.SessionID,
				TraceID:      state.TraceID,
				CheckpointID: state.Checkpoint,
				AllowRead:    state.Cache.AllowExact,
			})
			if err != nil {
				return nil, err
			}
			if result.CacheHit {
				state.Response = result.Response
				state.Session.CacheHitLevel = result.HitLevel
				state.Session.FinalAnswer = result.FinalAnswer
				state.Session.Intent = result.Intent
				state.Session.Route = result.Route
			}
			return state, nil
		}), compose.WithNodeName("L0ExactCacheNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("L1SemanticCacheNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			if b.L1SemanticCache == nil {
				return state, nil
			}
			result, err := b.L1SemanticCache.Invoke(ctx, globalnode.L1SemanticCacheInput{
				TenantID:     state.Session.TenantID,
				UserID:       state.Request.UserID,
				Query:        state.Session.RawQuery,
				SessionID:    state.Request.SessionID,
				TraceID:      state.TraceID,
				CheckpointID: state.Checkpoint,
				IntentBucket: state.Cache.IntentBucket,
				Scope:        cache.CacheScope(state.Cache.Scope),
				AllowRead:    state.Cache.AllowSemantic,
			})
			if err != nil {
				return nil, err
			}
			if result.CacheHit {
				state.Response = result.Response
				state.Session.CacheHitLevel = result.HitLevel
				state.Session.FinalAnswer = result.FinalAnswer
				state.Session.Intent = result.Intent
				state.Session.Route = result.Route
			}
			return state, nil
		}), compose.WithNodeName("L1SemanticCacheNode")); err != nil {
		return err
	}

	if b.QueryRewrite != nil {
		if err := g.AddLambdaNode("QueryRewriteNode", compose.InvokableLambda(b.QueryRewrite.Invoke), compose.WithNodeName("QueryRewriteNode")); err != nil {
			return err
		}
	}

	if b.IntentClassify != nil {
		if err := g.AddLambdaNode("IntentClassifyNode", compose.InvokableLambda(b.IntentClassify.Apply), compose.WithNodeName("IntentClassifyNode")); err != nil {
			return err
		}
	}

	if err := g.AddLambdaNode("PrepareReturnPolicyRAGInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (orchestratorragnode.Input, error) {
			if state == nil {
				return orchestratorragnode.Input{}, fmt.Errorf("state is nil")
			}
			return orchestratorragnode.Input{
				Message: state.Session.RawQuery,
				History: append([]*schema.Message(nil), state.Session.Messages...),
				Intent:  string(state.Session.Intent),
			}, nil
		}), compose.WithNodeName("PrepareReturnPolicyRAGInputNode")); err != nil {
		return err
	}

	if b.RAG != nil {
		if err := g.AddLambdaNode("ReturnPolicyRAGNode", compose.InvokableLambda(b.RAG.Invoke), compose.WithNodeName("ReturnPolicyRAGNode")); err != nil {
			return err
		}
	}

	if err := g.AddLambdaNode("ApplyReturnPolicyRAGResultNode", compose.InvokableLambda(
		func(ctx context.Context, result *orchestratorragnode.Result) (*orchestratorstate.State, error) {
			var current *orchestratorstate.State
			if err := orchestratorstate.ProcessState(ctx, func(state *orchestratorstate.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				if result == nil {
					return nil
				}
				state.Rewrite.Query = result.Query
				state.Rewrite.Reason = ""
				state.Retrieval.Documents = append([]*schema.Document(nil), result.Documents...)
				state.Answer.CacheableHint = boolPtr(true)
				return nil
			}); err != nil {
				return nil, err
			}
			if current == nil {
				return nil, fmt.Errorf("state is nil")
			}
			return current, nil
		}), compose.WithNodeName("ApplyReturnPolicyRAGResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareOrderQueryInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (orderquery.Input, error) {
			if state == nil {
				return orderquery.Input{}, fmt.Errorf("state is nil")
			}
			return orderquery.Input{Slots: cloneSlots(state.Session.Slots)}, nil
		}), compose.WithNodeName("PrepareOrderQueryInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyOrderQueryResultNode", compose.InvokableLambda(
		func(ctx context.Context, result orderquery.Output) (*orchestratorstate.State, error) {
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages, false)
		}), compose.WithNodeName("ApplyOrderQueryResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareInventoryInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (inventory.Input, error) {
			if state == nil {
				return inventory.Input{}, fmt.Errorf("state is nil")
			}
			return inventory.Input{Slots: cloneSlots(state.Session.Slots)}, nil
		}), compose.WithNodeName("PrepareInventoryInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyInventoryResultNode", compose.InvokableLambda(
		func(ctx context.Context, result inventory.Output) (*orchestratorstate.State, error) {
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages, false)
		}), compose.WithNodeName("ApplyInventoryResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareAddToCartInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (addtocart.Input, error) {
			if state == nil {
				return addtocart.Input{}, fmt.Errorf("state is nil")
			}
			return addtocart.Input{
				Slots:    cloneSlots(state.Session.Slots),
				Recorder: state.Recorder,
			}, nil
		}), compose.WithNodeName("PrepareAddToCartInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyAddToCartResultNode", compose.InvokableLambda(
		func(ctx context.Context, result addtocart.Output) (*orchestratorstate.State, error) {
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages, false)
		}), compose.WithNodeName("ApplyAddToCartResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareProductInfoInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (productinfo.Input, error) {
			if state == nil {
				return productinfo.Input{}, fmt.Errorf("state is nil")
			}
			return productinfo.Input{
				Slots:    cloneSlots(state.Session.Slots),
				RawQuery: state.Session.RawQuery,
				History:  append([]*schema.Message(nil), state.Session.Messages...),
				Intent:   string(state.Session.Intent),
				Recorder: state.Recorder,
			}, nil
		}), compose.WithNodeName("PrepareProductInfoInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyProductInfoResultNode", compose.InvokableLambda(
		func(ctx context.Context, result productinfo.Output) (*orchestratorstate.State, error) {
			var current *orchestratorstate.State
			if err := orchestratorstate.ProcessState(ctx, func(state *orchestratorstate.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Session.FinalAnswer = result.FinalAnswer
				state.Session.NeedHandoff = result.NeedHandoff
				state.Session.HandoffReason = result.HandoffReason
				state.Session.ReadOnly = result.ReadOnly
				state.Tool.Plans = nil
				state.Tool.CallMessage = nil
				state.Tool.ToolMessages = append([]*schema.Message(nil), result.ToolMessages...)
				support.HydrateToolResults(state)
				state.Rewrite.Query = result.Query
				state.Rewrite.Reason = ""
				state.Retrieval.Documents = append([]*schema.Document(nil), result.Documents...)
				state.Answer.CacheableHint = boolPtr(true)
				return nil
			}); err != nil {
				return nil, err
			}
			return current, nil
		}), compose.WithNodeName("ApplyProductInfoResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareReturnExchangeInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (returnexchange.Input, error) {
			if state == nil {
				return returnexchange.Input{}, fmt.Errorf("state is nil")
			}
			return returnexchange.Input{
				Slots:    cloneSlots(state.Session.Slots),
				Message:  state.Request.Message,
				Intent:   state.Session.Intent,
				Recorder: state.Recorder,
			}, nil
		}), compose.WithNodeName("PrepareReturnExchangeInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyReturnExchangeResultNode", compose.InvokableLambda(
		func(ctx context.Context, result returnexchange.Output) (*orchestratorstate.State, error) {
			var current *orchestratorstate.State
			if err := orchestratorstate.ProcessState(ctx, func(state *orchestratorstate.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Session.FinalAnswer = result.FinalAnswer
				state.Session.NeedHandoff = result.NeedHandoff
				state.Session.HandoffReason = result.HandoffReason
				state.Session.ReadOnly = result.ReadOnly
				state.Session.AwaitingConfirm = result.AwaitingConfirm
				state.Tool.Plans = nil
				state.Tool.CallMessage = nil
				state.Tool.ToolMessages = append([]*schema.Message(nil), result.ToolMessages...)
				support.HydrateToolResults(state)
				state.Answer.CacheableHint = boolPtr(false)
				return nil
			}); err != nil {
				return nil, err
			}
			if current != nil && result.AwaitingConfirm {
				reply := result.FinalAnswer
				resp := current.EnsureResponse()
				resp.Reply = reply
				resp.Intent = current.Session.Intent
				resp.Status = domain.ReplyStatusFallback
				resp.Confidence = 0.9
				current.Session.FinalAnswer = reply
				current.Answer.CacheableHint = boolPtr(false)
				current.Interrupt = &orchestratorstate.InterruptState{
					Payload: map[string]any{"confirm": true, "message": reply},
				}
				return current, nil
			}
			return current, nil
		}), compose.WithNodeName("ApplyReturnExchangeResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareBaseQAInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (baseqa.Input, error) {
			if state == nil {
				return baseqa.Input{}, fmt.Errorf("state is nil")
			}
			return baseqa.Input{
				RawQuery:    state.Session.RawQuery,
				Intent:      string(state.Session.Intent),
				History:     append([]*schema.Message(nil), state.Session.Messages...),
				FinalAnswer: state.Session.FinalAnswer,
			}, nil
		}), compose.WithNodeName("PrepareBaseQAInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyBaseQAResultNode", compose.InvokableLambda(
		func(ctx context.Context, result baseqa.Output) (*orchestratorstate.State, error) {
			var current *orchestratorstate.State
			if err := orchestratorstate.ProcessState(ctx, func(state *orchestratorstate.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Session.FinalAnswer = result.FinalAnswer
				state.Rewrite.Query = result.Query
				state.Rewrite.Reason = ""
				state.Retrieval.Documents = append([]*schema.Document(nil), result.Documents...)
				state.Answer.CacheableHint = boolPtr(false)
				return nil
			}); err != nil {
				return nil, err
			}
			return current, nil
		}), compose.WithNodeName("ApplyBaseQAResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("GlobalSlotExtractNode", compose.InvokableLambda(b.GlobalSlotExtract.Apply), compose.WithNodeName("GlobalSlotExtractNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("GlobalSlotCheckNode", compose.InvokableLambda(b.GlobalSlotCheck.Apply), compose.WithNodeName("GlobalSlotCheckNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("AskUserNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			result, err := b.AskUser.Invoke(ctx, globalnode.AskUserInput{
				Reply:            state.Session.FinalAnswer,
				Intent:           state.Session.Intent,
				IntentConfidence: state.Session.IntentConfidence,
				MissingSlots:     state.Session.MissingSlots,
			})
			if err != nil {
				return nil, err
			}
			resp := state.EnsureResponse()
			resp.Reply = result.Reply
			resp.Intent = result.Intent
			resp.Status = domain.ReplyStatusFallback
			resp.Confidence = result.Confidence
			state.Session.FinalAnswer = result.Reply
			state.Session.AwaitingUser = true
			state.Answer.CacheableHint = boolPtr(false)
			state.Interrupt = &orchestratorstate.InterruptState{
				Payload: map[string]any{
					"missing_slots": result.MissingSlots,
					"question":      result.Reply,
				},
			}
			return state, nil
		}), compose.WithNodeName("AskUserNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("RouteNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			result, err := b.Route.Invoke(ctx, globalnode.RouteInput{
				Intent:          state.Intent,
				FeatureFlags:    state.Session.FeatureFlags,
				AwaitingConfirm: state.Session.AwaitingConfirm,
			})
			if err != nil {
				return nil, err
			}
			session := &state.Session
			session.Route = result.Route
			session.ErrorCode = result.ErrorCode
			session.ReadOnly = result.ReadOnly
			return state, nil
		}), compose.WithNodeName("RouteNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareSkillSelectInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.State) (globalnode.SkillSelectInput, error) {
			if state == nil {
				return globalnode.SkillSelectInput{}, fmt.Errorf("state is nil")
			}
			return globalnode.SkillSelectInput{
				Route: state.Session.Route,
			}, nil
		}), compose.WithNodeName("PrepareSkillSelectInputNode")); err != nil {
		return err
	}

	if b.SkillSelect != nil {
		if err := g.AddLambdaNode("SkillSelectNode", compose.InvokableLambda(b.SkillSelect.Invoke), compose.WithNodeName("SkillSelectNode")); err != nil {
			return err
		}
	}

	if err := g.AddLambdaNode("ApplySkillSelectResultNode", compose.InvokableLambda(
		func(ctx context.Context, result *globalnode.SkillSelectResult) (*orchestratorstate.State, error) {
			var current *orchestratorstate.State
			if err := orchestratorstate.ProcessState(ctx, func(state *orchestratorstate.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Skill.Names = nil
				if result != nil {
					state.Skill.Names = append([]string(nil), result.Names...)
				}
				return nil
			}); err != nil {
				return nil, err
			}
			if current == nil {
				return nil, fmt.Errorf("state is nil")
			}
			return current, nil
		}), compose.WithNodeName("ApplySkillSelectResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("FinalizeNode", compose.InvokableLambda(b.Finalize.Invoke), compose.WithNodeName("FinalizeNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("InterruptNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.State) (*orchestratorstate.State, error) {
			if state == nil || state.Interrupt == nil || len(state.Interrupt.Payload) == 0 {
				return state, nil
			}
			return state, compose.Interrupt(ctx, cloneSlots(state.Interrupt.Payload))
		}), compose.WithNodeName("InterruptNode")); err != nil {
		return err
	}
	return nil
}

func (b *Builder) addSubgraphs(ctx context.Context, g *compose.Graph[map[string]any, *orchestratorstate.State]) error {
	type subgraphEntry struct {
		name  string
		build func() (compose.AnyGraph, error)
	}

	subgraphs := []subgraphEntry{
		{"OrderQueryGraph", func() (compose.AnyGraph, error) { return orderquery.Build(ctx, b.Registry, b.OrderRead) }},
		{"InventoryGraph", func() (compose.AnyGraph, error) { return inventory.Build(ctx, b.Registry, b.InventoryRead) }},
		{"ProductInfoGraph", func() (compose.AnyGraph, error) { return productinfo.Build(ctx, b.Registry, b.ProductInfo, b.RAG) }},
		{"AddToCartGraph", func() (compose.AnyGraph, error) { return addtocart.Build(ctx, b.Registry, b.AddToCart) }},
		{"ReturnExchangeGraph", func() (compose.AnyGraph, error) {
			return returnexchange.Build(ctx, b.Registry, b.ReturnExchangeQuery, b.EligibilityCheck, b.ConfirmSummary, b.SubmitAfterSale)
		}},
		{"BaseQAGraph", func() (compose.AnyGraph, error) { return baseqa.Build(ctx, b.RAG, b.BaseQA) }},
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

func (b *Builder) addEdges(g *compose.Graph[map[string]any, *orchestratorstate.State]) error {
	edges := [][2]string{
		{compose.START, "AccessGuardNode"},
		{"SessionLoadNode", "CachePreCheckNode"},
		{"CachePreCheckNode", "L0ExactCacheNode"},
		{"L0ExactCacheNode", "L1SemanticCacheNode"},
		{"QueryRewriteNode", "IntentClassifyNode"},
		{"IntentClassifyNode", "GlobalSlotExtractNode"},
		{"GlobalSlotExtractNode", "GlobalSlotCheckNode"},

		{"AskUserNode", "FinalizeNode"},
		{"PrepareSkillSelectInputNode", "SkillSelectNode"},
		{"SkillSelectNode", "ApplySkillSelectResultNode"},
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
		{"PrepareReturnPolicyRAGInputNode", "ReturnPolicyRAGNode"},
		{"ReturnPolicyRAGNode", "ApplyReturnPolicyRAGResultNode"},
		{"ApplyReturnPolicyRAGResultNode", "FinalizeNode"},
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

func (b *Builder) addBranches(g *compose.Graph[map[string]any, *orchestratorstate.State]) error {
	if err := g.AddBranch("AccessGuardNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.State) (string, error) {
			var state *orchestratorstate.State
			_ = compose.ProcessState(ctx, func(_ context.Context, current *orchestratorstate.State) error {
				state = current
				return nil
			})
			if state != nil && state.Session.ErrorCode != "" {
				return "FinalizeNode", nil
			}
			return "SessionLoadNode", nil
		},
		map[string]bool{"FinalizeNode": true, "SessionLoadNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("L0ExactCacheNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.State) (string, error) {
			var state *orchestratorstate.State
			_ = compose.ProcessState(ctx, func(_ context.Context, current *orchestratorstate.State) error {
				state = current
				return nil
			})
			if state != nil && state.Session.CacheHitLevel != "" {
				return "FinalizeNode", nil
			}
			return "L1SemanticCacheNode", nil
		},
		map[string]bool{"FinalizeNode": true, "L1SemanticCacheNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("L1SemanticCacheNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.State) (string, error) {
			var state *orchestratorstate.State
			_ = compose.ProcessState(ctx, func(_ context.Context, current *orchestratorstate.State) error {
				state = current
				return nil
			})
			if state != nil && state.Session.CacheHitLevel != "" {
				return "FinalizeNode", nil
			}
			return "QueryRewriteNode", nil
		},
		map[string]bool{"FinalizeNode": true, "QueryRewriteNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("GlobalSlotCheckNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.State) (string, error) {
			var state *orchestratorstate.State
			_ = compose.ProcessState(ctx, func(_ context.Context, current *orchestratorstate.State) error {
				state = current
				return nil
			})
			if state != nil && state.Session.AwaitingUser {
				return "AskUserNode", nil
			}
			return "RouteNode", nil
		},
		map[string]bool{"AskUserNode": true, "RouteNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("RouteNode", compose.NewGraphBranch(
		func(_ context.Context, _ *orchestratorstate.State) (string, error) {
			return "PrepareSkillSelectInputNode", nil
		},
		map[string]bool{"PrepareSkillSelectInputNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("FinalizeNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.State) (string, error) {
			var state *orchestratorstate.State
			_ = compose.ProcessState(ctx, func(_ context.Context, current *orchestratorstate.State) error {
				state = current
				return nil
			})
			if state != nil && state.Interrupt != nil && len(state.Interrupt.Payload) > 0 {
				return "InterruptNode", nil
			}
			return compose.END, nil
		},
		map[string]bool{"InterruptNode": true, compose.END: true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("ApplySkillSelectResultNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.State) (string, error) {
			var state *orchestratorstate.State
			_ = compose.ProcessState(ctx, func(_ context.Context, current *orchestratorstate.State) error {
				state = current
				return nil
			})
			if state == nil {
				return "PrepareBaseQAInputNode", nil
			}
			switch state.Session.Route {
			case orchestratorstate.RouteOrderQuery:
				return "PrepareOrderQueryInputNode", nil
			case orchestratorstate.RouteInventory:
				return "PrepareInventoryInputNode", nil
			case orchestratorstate.RouteProductInfo:
				return "PrepareProductInfoInputNode", nil
			case orchestratorstate.RouteAddToCart:
				return "PrepareAddToCartInputNode", nil
			case orchestratorstate.RouteReturnPolicy:
				return "PrepareReturnPolicyRAGInputNode", nil
			case orchestratorstate.RouteReturnExchangeApply:
				return "PrepareReturnExchangeInputNode", nil
			default:
				return "PrepareBaseQAInputNode", nil
			}
		},
		map[string]bool{
			"PrepareOrderQueryInputNode":      true,
			"PrepareInventoryInputNode":       true,
			"PrepareProductInfoInputNode":     true,
			"PrepareAddToCartInputNode":       true,
			"PrepareReturnPolicyRAGInputNode": true,
			"PrepareReturnExchangeInputNode":  true,
			"PrepareBaseQAInputNode":          true,
		},
	)); err != nil {
		return err
	}
	return nil
}

func applyToolFlowResult(
	ctx context.Context,
	finalAnswer string,
	needHandoff bool,
	handoffReason string,
	readOnly bool,
	toolMessages []*schema.Message,
	cacheableHint bool,
) (*orchestratorstate.State, error) {
	var current *orchestratorstate.State
	if err := orchestratorstate.ProcessState(ctx, func(state *orchestratorstate.State) error {
		if state == nil {
			return fmt.Errorf("state is nil")
		}
		current = state
		state.Session.FinalAnswer = finalAnswer
		state.Session.NeedHandoff = needHandoff
		state.Session.HandoffReason = handoffReason
		state.Session.ReadOnly = readOnly
		state.Tool.Plans = nil
		state.Tool.CallMessage = nil
		state.Tool.ToolMessages = append([]*schema.Message(nil), toolMessages...)
		state.Answer.CacheableHint = boolPtr(cacheableHint)
		support.HydrateToolResults(state)
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

func clonePendingSelectionsState(input map[string]orchestratorstate.PendingSelection) map[string]orchestratorstate.PendingSelection {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]orchestratorstate.PendingSelection, len(input))
	for key, selection := range input {
		cloned := orchestratorstate.PendingSelection{Kind: selection.Kind}
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
