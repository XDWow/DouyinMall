package graph

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/rag"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/fallback"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnexchange"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// Config 控制主图级别的中断行为。
type Config struct {
	InterruptBeforeNodes []string
	InterruptAfterNodes  []string
}

// Builder 负责组装主图和业务子图。
type Builder struct {
	Config          Config
	CheckpointStore cache.CheckpointStore

	Registry *agenttool.Registry

	AccessGuard     *orchestratornode.AccessGuardNode
	SessionLoad     *orchestratornode.SessionLoadNode
	CachePolicy     *orchestratornode.CachePolicyNode
	MultiLevelCache *orchestratornode.MultiLevelCacheNode
	IntentClassify  *orchestratornode.IntentClassifyNode
	SlotExtract     *orchestratornode.SlotExtractNode
	SlotCheck       *orchestratornode.SlotCheckNode
	AskUser         *orchestratornode.AskUserNode
	Route           *orchestratornode.RouteNode
	SkillSelect     *orchestratornode.SkillSelectNode
	ResponseRender  *orchestratornode.ResponseRenderNode
	CacheWriteback  *orchestratornode.CacheWritebackNode

	OrderRead           *orchestratornode.OrderReadNode
	InventoryRead       *orchestratornode.InventoryReadNode
	ProductInfo         *orchestratornode.ProductInfoNode
	AddToCart           *orchestratornode.AddToCartNode
	ReturnExchangeQuery *orchestratornode.ReturnExchangeQueryNode
	EligibilityCheck    *orchestratornode.EligibilityCheckNode
	ConfirmSummary      *orchestratornode.ConfirmSummaryNode
	SubmitAfterSale     *orchestratornode.SubmitAfterSaleNode
	RAG                 *orchestratorragnode.RAGNode
	Fallback            *orchestratornode.FallbackNode
}

func (b *Builder) Build(ctx context.Context) (compose.Runnable[map[string]any, *orchestratorstate.ConversationState], error) {
	g := compose.NewGraph[map[string]any, *orchestratorstate.ConversationState](
		compose.WithGenLocalState(func(context.Context) *orchestratorstate.ConversationState {
			return &orchestratorstate.ConversationState{}
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

func (b *Builder) addPipelineNodes(g *compose.Graph[map[string]any, *orchestratorstate.ConversationState]) error {
	if err := g.AddLambdaNode("AccessGuardNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.AccessGuard.Invoke(ctx, orchestratornode.AccessGuardInput{
				Message:     state.Request.Message,
				UserID:      state.Request.UserID,
				ResumeToken: state.Request.ResumeToken,
			})
			if err != nil {
				return nil, err
			}
			session := orchestratorstate.EnsureSessionState(state)
			session.UserID = result.UserID
			session.RawQuery = result.RawQuery
			session.TenantID = result.TenantID
			session.ResumeFromCP = result.ResumeFromCP
			session.NeedHandoff = result.NeedHandoff
			session.HandoffReason = result.HandoffReason
			session.FinalAnswer = result.FinalAnswer
			session.Route = result.Route
			return state, nil
		}), compose.WithNodeName("AccessGuardNode"), compose.WithInputKey("flow")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("SessionLoadNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.SessionLoad.Invoke(ctx, orchestratornode.SessionLoadInput{
				Request:     state.Request,
				TraceID:     state.TraceID,
				SessionMeta: state.SessionMeta,
			})
			if err != nil {
				return nil, err
			}
			session := orchestratorstate.EnsureSessionState(state)
			state.SessionMeta = result.SessionMeta
			session.SessionID = result.SessionID
			if orchestratorstate.SlotString(state, "order_id") == "" && result.OrderID != "" {
				orchestratorstate.SetSlot(state, "order_id", result.OrderID)
			}
			if orchestratorstate.SlotString(state, "product_id") == "" && result.ProductID != "" {
				orchestratorstate.SetSlot(state, "product_id", result.ProductID)
			}
			state.EnsureResponse().SessionID = result.SessionID
			return state, nil
		}), compose.WithNodeName("SessionLoadNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("CachePolicyNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.CachePolicy.Invoke(ctx, orchestratornode.CachePolicyInput{
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
		}), compose.WithNodeName("CachePolicyNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("MultiLevelCacheNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.MultiLevelCache.Invoke(ctx, orchestratornode.MultiLevelCacheInput{
				TenantID:     state.Session.TenantID,
				UserID:       state.Request.UserID,
				RawQuery:     state.Session.RawQuery,
				SessionID:    state.Request.SessionID,
				TraceID:      state.TraceID,
				CheckpointID: state.Checkpoint,
				Policy: orchestratornode.CachePolicyResult{
					AllowExact:    state.Cache.AllowExact,
					AllowSemantic: state.Cache.AllowSemantic,
					IntentBucket:  state.Cache.IntentBucket,
					Scope:         cache.CacheScope(state.Cache.Scope),
				},
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
		}), compose.WithNodeName("MultiLevelCacheNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareIntentClassifyInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (orchestratornode.IntentClassifyInput, error) {
			if state == nil {
				return orchestratornode.IntentClassifyInput{}, fmt.Errorf("对话状态不能为空")
			}
			return orchestratornode.IntentClassifyInput{
				Message: state.Session.RawQuery,
				History: orchestratorstate.RecentMessages(state),
			}, nil
		}), compose.WithNodeName("PrepareIntentClassifyInputNode")); err != nil {
		return err
	}

	if b.IntentClassify != nil {
		if err := g.AddLambdaNode("IntentClassifyNode", compose.InvokableLambda(b.IntentClassify.Invoke), compose.WithNodeName("IntentClassifyNode")); err != nil {
			return err
		}
	}

	if err := g.AddLambdaNode("ApplyIntentClassifyResultNode", compose.InvokableLambda(
		func(ctx context.Context, result *orchestratornode.IntentClassifyResult) (*orchestratorstate.ConversationState, error) {
			var current *orchestratorstate.ConversationState
			if err := orchestratorstate.ProcessConversationState(ctx, func(state *orchestratorstate.ConversationState) error {
				if state == nil {
					return fmt.Errorf("对话状态不能为空")
				}
				current = state
				if result == nil {
					return nil
				}
				state.Intent.Intent = result.Intent
				state.Intent.Confidence = result.Confidence
				state.Intent.Entities = result.Entities
				state.Intent.NeedRewrite = result.NeedRewrite
				state.Intent.Reason = result.Reason
				state.Session.Intent = result.Intent
				state.Session.IntentConfidence = result.Confidence
				return nil
			}); err != nil {
				return nil, err
			}
			if current == nil {
				return nil, fmt.Errorf("对话状态不能为空")
			}
			return current, nil
		}), compose.WithNodeName("ApplyIntentClassifyResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareReturnPolicyRAGInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (orchestratorragnode.Input, error) {
			if state == nil {
				return orchestratorragnode.Input{}, fmt.Errorf("对话状态不能为空")
			}
			return orchestratorragnode.Input{
				Message: state.Session.RawQuery,
				History: orchestratorstate.RecentMessages(state),
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
		func(ctx context.Context, result *orchestratorragnode.Result) (*orchestratorstate.ConversationState, error) {
			var current *orchestratorstate.ConversationState
			if err := orchestratorstate.ProcessConversationState(ctx, func(state *orchestratorstate.ConversationState) error {
				if state == nil {
					return fmt.Errorf("对话状态不能为空")
				}
				current = state
				if result == nil {
					return nil
				}
				state.Rewrite.Query = result.Query
				state.Rewrite.Reason = ""
				state.Retrieval.Documents = append([]*schema.Document(nil), result.Documents...)
				return nil
			}); err != nil {
				return nil, err
			}
			if current == nil {
				return nil, fmt.Errorf("对话状态不能为空")
			}
			return current, nil
		}), compose.WithNodeName("ApplyReturnPolicyRAGResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareOrderQueryInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (orderquery.Input, error) {
			if state == nil {
				return orderquery.Input{}, fmt.Errorf("对话状态不能为空")
			}
			return orderquery.Input{Slots: cloneSlots(state.Session.Slots)}, nil
		}), compose.WithNodeName("PrepareOrderQueryInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyOrderQueryResultNode", compose.InvokableLambda(
		func(ctx context.Context, result orderquery.Output) (*orchestratorstate.ConversationState, error) {
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages)
		}), compose.WithNodeName("ApplyOrderQueryResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareInventoryInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (inventory.Input, error) {
			if state == nil {
				return inventory.Input{}, fmt.Errorf("对话状态不能为空")
			}
			return inventory.Input{Slots: cloneSlots(state.Session.Slots)}, nil
		}), compose.WithNodeName("PrepareInventoryInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyInventoryResultNode", compose.InvokableLambda(
		func(ctx context.Context, result inventory.Output) (*orchestratorstate.ConversationState, error) {
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages)
		}), compose.WithNodeName("ApplyInventoryResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareAddToCartInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (addtocart.Input, error) {
			if state == nil {
				return addtocart.Input{}, fmt.Errorf("对话状态不能为空")
			}
			return addtocart.Input{
				Slots:    cloneSlots(state.Session.Slots),
				Recorder: state.Recorder,
			}, nil
		}), compose.WithNodeName("PrepareAddToCartInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyAddToCartResultNode", compose.InvokableLambda(
		func(ctx context.Context, result addtocart.Output) (*orchestratorstate.ConversationState, error) {
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages)
		}), compose.WithNodeName("ApplyAddToCartResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareProductInfoInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (productinfo.Input, error) {
			if state == nil {
				return productinfo.Input{}, fmt.Errorf("对话状态不能为空")
			}
			return productinfo.Input{
				Slots:    cloneSlots(state.Session.Slots),
				RawQuery: state.Session.RawQuery,
				History:  orchestratorstate.RecentMessages(state),
				Intent:   string(state.Session.Intent),
				Recorder: state.Recorder,
			}, nil
		}), compose.WithNodeName("PrepareProductInfoInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyProductInfoResultNode", compose.InvokableLambda(
		func(ctx context.Context, result productinfo.Output) (*orchestratorstate.ConversationState, error) {
			var current *orchestratorstate.ConversationState
			if err := orchestratorstate.ProcessConversationState(ctx, func(state *orchestratorstate.ConversationState) error {
				if state == nil {
					return fmt.Errorf("对话状态不能为空")
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
				return nil
			}); err != nil {
				return nil, err
			}
			return current, nil
		}), compose.WithNodeName("ApplyProductInfoResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareReturnExchangeInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (returnexchange.Input, error) {
			if state == nil {
				return returnexchange.Input{}, fmt.Errorf("对话状态不能为空")
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
		func(ctx context.Context, result returnexchange.Output) (*orchestratorstate.ConversationState, error) {
			var current *orchestratorstate.ConversationState
			if err := orchestratorstate.ProcessConversationState(ctx, func(state *orchestratorstate.ConversationState) error {
				if state == nil {
					return fmt.Errorf("对话状态不能为空")
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
				if b.ConfirmSummary != nil && b.ConfirmSummary.PersistTurn != nil {
					if err := b.ConfirmSummary.PersistTurn(ctx, current, reply, resp.Intent, resp.Confidence); err != nil && b.ConfirmSummary.Logger != nil {
						b.ConfirmSummary.Logger.Warn("持久化确认轮次失败", logger.Error(err))
					}
				}
				return current, compose.Interrupt(ctx, map[string]any{"confirm": true, "message": reply})
			}
			return current, nil
		}), compose.WithNodeName("ApplyReturnExchangeResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareFallbackInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (fallback.Input, error) {
			if state == nil {
				return fallback.Input{}, fmt.Errorf("对话状态不能为空")
			}
			return fallback.Input{
				RawQuery:    state.Session.RawQuery,
				Intent:      string(state.Session.Intent),
				History:     orchestratorstate.RecentMessages(state),
				FinalAnswer: state.Session.FinalAnswer,
			}, nil
		}), compose.WithNodeName("PrepareFallbackInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyFallbackResultNode", compose.InvokableLambda(
		func(ctx context.Context, result fallback.Output) (*orchestratorstate.ConversationState, error) {
			var current *orchestratorstate.ConversationState
			if err := orchestratorstate.ProcessConversationState(ctx, func(state *orchestratorstate.ConversationState) error {
				if state == nil {
					return fmt.Errorf("对话状态不能为空")
				}
				current = state
				state.Session.FinalAnswer = result.FinalAnswer
				state.Rewrite.Query = result.Query
				state.Rewrite.Reason = ""
				state.Retrieval.Documents = append([]*schema.Document(nil), result.Documents...)
				return nil
			}); err != nil {
				return nil, err
			}
			return current, nil
		}), compose.WithNodeName("ApplyFallbackResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("SlotExtractNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.SlotExtract.Invoke(ctx, orchestratornode.SlotExtractInput{
				ExistingSlots:   state.Session.Slots,
				RequestMetadata: state.Request.Metadata,
				Intent:          state.Session.Intent,
				IntentEntities:  state.Intent.Entities,
				RawQuery:        state.Session.RawQuery,
				AwaitingUser:    state.Session.AwaitingUser,
				AwaitingConfirm: state.Session.AwaitingConfirm,
				ResumeFromCP:    state.Session.ResumeFromCP,
			})
			if err != nil {
				return nil, err
			}
			session := orchestratorstate.EnsureSessionState(state)
			session.Slots = result.Slots
			session.AwaitingUser = result.AwaitingUser
			session.AwaitingConfirm = result.AwaitingConfirm
			return state, nil
		}), compose.WithNodeName("SlotExtractNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("SlotCheckNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.SlotCheck.Invoke(ctx, orchestratornode.SlotCheckInput{
				Intent:          state.Session.Intent,
				Slots:           state.Session.Slots,
				RawQuery:        state.Session.RawQuery,
				AwaitingConfirm: state.Session.AwaitingConfirm,
				NeedHandoff:     state.Session.NeedHandoff,
			})
			if err != nil {
				return nil, err
			}
			session := orchestratorstate.EnsureSessionState(state)
			session.MissingSlots = result.MissingSlots
			session.AwaitingUser = result.AwaitingUser
			if result.FinalAnswer != "" {
				session.FinalAnswer = result.FinalAnswer
			}
			return state, nil
		}), compose.WithNodeName("SlotCheckNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("AskUserNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.AskUser.Invoke(ctx, orchestratornode.AskUserInput{
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
			if b.AskUser.PersistTurn != nil {
				if err := b.AskUser.PersistTurn(ctx, state, result.Reply, result.Intent, result.Confidence); err != nil {
					b.AskUser.Logger.Warn("持久化补槽追问失败", logger.Error(err))
				}
			}
			return state, compose.Interrupt(ctx, map[string]any{
				"missing_slots": result.MissingSlots,
				"question":      result.Reply,
			})
		}), compose.WithNodeName("AskUserNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("RouteNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			result, err := b.Route.Invoke(ctx, orchestratornode.RouteInput{
				Intent:          state.Intent,
				FeatureFlags:    state.Session.FeatureFlags,
				AwaitingConfirm: state.Session.AwaitingConfirm,
			})
			if err != nil {
				return nil, err
			}
			session := orchestratorstate.EnsureSessionState(state)
			session.Route = result.Route
			session.ErrorCode = result.ErrorCode
			session.ReadOnly = result.ReadOnly
			return state, nil
		}), compose.WithNodeName("RouteNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareSkillSelectInputNode", compose.InvokableLambda(
		func(_ context.Context, state *orchestratorstate.ConversationState) (orchestratornode.SkillSelectInput, error) {
			if state == nil {
				return orchestratornode.SkillSelectInput{}, fmt.Errorf("对话状态不能为空")
			}
			return orchestratornode.SkillSelectInput{
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
		func(ctx context.Context, result *orchestratornode.SkillSelectResult) (*orchestratorstate.ConversationState, error) {
			var current *orchestratorstate.ConversationState
			if err := orchestratorstate.ProcessConversationState(ctx, func(state *orchestratorstate.ConversationState) error {
				if state == nil {
					return fmt.Errorf("对话状态不能为空")
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
				return nil, fmt.Errorf("对话状态不能为空")
			}
			return current, nil
		}), compose.WithNodeName("ApplySkillSelectResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("ResponseRenderNode", compose.InvokableLambda(b.ResponseRender.Invoke), compose.WithNodeName("ResponseRenderNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("CacheWritebackNode", compose.InvokableLambda(b.CacheWriteback.Invoke), compose.WithNodeName("CacheWritebackNode")); err != nil {
		return err
	}
	return nil
}

func (b *Builder) addSubgraphs(ctx context.Context, g *compose.Graph[map[string]any, *orchestratorstate.ConversationState]) error {
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
		{"FallbackGraph", func() (compose.AnyGraph, error) { return fallback.Build(ctx, b.RAG, b.Fallback) }},
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

// 工作流看这里
func (b *Builder) addEdges(g *compose.Graph[map[string]any, *orchestratorstate.ConversationState]) error {
	edges := [][2]string{
		{compose.START, "AccessGuardNode"},
		{"AccessGuardNode", "SessionLoadNode"},
		{"SessionLoadNode", "CachePolicyNode"},
		{"CachePolicyNode", "MultiLevelCacheNode"},
		{"PrepareIntentClassifyInputNode", "IntentClassifyNode"},
		{"IntentClassifyNode", "ApplyIntentClassifyResultNode"},
		{"ApplyIntentClassifyResultNode", "SlotExtractNode"},
		{"SlotExtractNode", "SlotCheckNode"},
		{"AskUserNode", compose.END},
		{"PrepareSkillSelectInputNode", "SkillSelectNode"},
		{"SkillSelectNode", "ApplySkillSelectResultNode"},
		{"PrepareOrderQueryInputNode", "OrderQueryGraph"},
		{"OrderQueryGraph", "ApplyOrderQueryResultNode"},
		{"ApplyOrderQueryResultNode", "ResponseRenderNode"},
		{"PrepareInventoryInputNode", "InventoryGraph"},
		{"InventoryGraph", "ApplyInventoryResultNode"},
		{"ApplyInventoryResultNode", "ResponseRenderNode"},
		{"PrepareProductInfoInputNode", "ProductInfoGraph"},
		{"ProductInfoGraph", "ApplyProductInfoResultNode"},
		{"ApplyProductInfoResultNode", "ResponseRenderNode"},
		{"PrepareAddToCartInputNode", "AddToCartGraph"},
		{"AddToCartGraph", "ApplyAddToCartResultNode"},
		{"ApplyAddToCartResultNode", "ResponseRenderNode"},
		{"PrepareReturnPolicyRAGInputNode", "ReturnPolicyRAGNode"},
		{"ReturnPolicyRAGNode", "ApplyReturnPolicyRAGResultNode"},
		{"ApplyReturnPolicyRAGResultNode", "ResponseRenderNode"},
		{"PrepareReturnExchangeInputNode", "ReturnExchangeGraph"},
		{"ReturnExchangeGraph", "ApplyReturnExchangeResultNode"},
		{"ApplyReturnExchangeResultNode", "ResponseRenderNode"},
		{"PrepareFallbackInputNode", "FallbackGraph"},
		{"FallbackGraph", "ApplyFallbackResultNode"},
		{"ApplyFallbackResultNode", "ResponseRenderNode"},
		{"ResponseRenderNode", "CacheWritebackNode"},
		{"CacheWritebackNode", compose.END},
	}
	for _, edge := range edges {
		if err := g.AddEdge(edge[0], edge[1]); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) addBranches(g *compose.Graph[map[string]any, *orchestratorstate.ConversationState]) error {
	if err := g.AddBranch("MultiLevelCacheNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state != nil && state.Session.CacheHitLevel != "" {
				return "ResponseRenderNode", nil
			}
			return "PrepareIntentClassifyInputNode", nil
		},
		map[string]bool{"ResponseRenderNode": true, "PrepareIntentClassifyInputNode": true},
	)); err != nil {
		return err
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
		return err
	}

	if err := g.AddBranch("RouteNode", compose.NewGraphBranch(
		func(_ context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			return "PrepareSkillSelectInputNode", nil
		},
		map[string]bool{"PrepareSkillSelectInputNode": true},
	)); err != nil {
		return err
	}

	if err := g.AddBranch("ApplySkillSelectResultNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state == nil {
				return "PrepareFallbackInputNode", nil
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
				return "PrepareFallbackInputNode", nil
			}
		},
		map[string]bool{
			"PrepareOrderQueryInputNode":      true,
			"PrepareInventoryInputNode":       true,
			"PrepareProductInfoInputNode":     true,
			"PrepareAddToCartInputNode":       true,
			"PrepareReturnPolicyRAGInputNode": true,
			"PrepareReturnExchangeInputNode":  true,
			"PrepareFallbackInputNode":        true,
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
) (*orchestratorstate.ConversationState, error) {
	var current *orchestratorstate.ConversationState
	if err := orchestratorstate.ProcessConversationState(ctx, func(state *orchestratorstate.ConversationState) error {
		if state == nil {
			return fmt.Errorf("对话状态不能为空")
		}
		current = state
		state.Session.FinalAnswer = finalAnswer
		state.Session.NeedHandoff = needHandoff
		state.Session.HandoffReason = handoffReason
		state.Session.ReadOnly = readOnly
		state.Tool.Plans = nil
		state.Tool.CallMessage = nil
		state.Tool.ToolMessages = append([]*schema.Message(nil), toolMessages...)
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
