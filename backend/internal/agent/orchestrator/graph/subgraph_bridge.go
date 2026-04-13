package graph

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart"
	baseqa "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/fallback"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnexchange"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnpolicy"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// addSubgraphBridgeNodes 注册主图与子图之间的 Eino 边界节点：
// Prepare* 将共享 *domain.State 投影为各子图入口类型 GraphInput；Apply* 将子图 Output 合并回 State。
func (b *Builder) addSubgraphBridgeNodes(g *compose.Graph[map[string]any, *domain.State]) error {
	if err := g.AddLambdaNode("PrepareReturnPolicyInputNode", compose.InvokableLambda(
		func(_ context.Context, state *domain.State) (returnpolicy.GraphInput, error) {
			return returnpolicy.InputFromState(state, b.Skills)
		}),
		compose.WithNodeName("PrepareReturnPolicyInputNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("ApplyReturnPolicyResultNode", compose.InvokableLambda(
		func(ctx context.Context, result returnpolicy.Output) (*domain.State, error) {
			var current *domain.State
			if err := domain.ProcessState(ctx, func(state *domain.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Session.AwaitingUser = false
				state.Session.MissingSlots = nil
				if result.CacheHit {
					state.Response = result.Response
					state.Session.CacheHitLevel = result.HitLevel
					state.Session.FinalAnswer = result.FinalAnswer
					if result.Response != nil {
						state.Session.Intent = result.Response.Intent
					}
					state.Session.Route = domain.RouteReturnPolicy
					return nil
				}
				state.Session.FinalAnswer = result.FinalAnswer
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
		}), compose.WithNodeName("ApplyReturnPolicyResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareOrderQueryInputNode", compose.InvokableLambda(
		func(_ context.Context, state *domain.State) (orderquery.GraphInput, error) {
			return orderquery.InputFromState(state)
		}),
		compose.WithNodeName("PrepareOrderQueryInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyOrderQueryResultNode", compose.InvokableLambda(
		func(ctx context.Context, result orderquery.Output) (*domain.State, error) {
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages, false)
		}), compose.WithNodeName("ApplyOrderQueryResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareInventoryInputNode", compose.InvokableLambda(
		func(_ context.Context, state *domain.State) (inventory.GraphInput, error) {
			return inventory.InputFromState(state)
		}),
		compose.WithNodeName("PrepareInventoryInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyInventoryResultNode", compose.InvokableLambda(
		func(ctx context.Context, result inventory.Output) (*domain.State, error) {
			if result.AwaitingUser {
				return applySubgraphSlotWait(ctx, b, result.MissingSlots, result.FinalAnswer)
			}
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages, false)
		}), compose.WithNodeName("ApplyInventoryResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareAddToCartInputNode", compose.InvokableLambda(
		func(_ context.Context, state *domain.State) (addtocart.GraphInput, error) {
			return addtocart.InputFromState(state)
		}),
		compose.WithNodeName("PrepareAddToCartInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyAddToCartResultNode", compose.InvokableLambda(
		func(ctx context.Context, result addtocart.Output) (*domain.State, error) {
			if result.AwaitingUser {
				return applySubgraphSlotWait(ctx, b, result.MissingSlots, result.FinalAnswer)
			}
			return applyToolFlowResult(ctx, result.FinalAnswer, result.NeedHandoff, result.HandoffReason, result.ReadOnly, result.ToolMessages, false)
		}), compose.WithNodeName("ApplyAddToCartResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareProductInfoInputNode", compose.InvokableLambda(
		func(_ context.Context, state *domain.State) (productinfo.GraphInput, error) {
			return productinfo.InputFromState(state, b.Skills)
		}),
		compose.WithNodeName("PrepareProductInfoInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyProductInfoResultNode", compose.InvokableLambda(
		func(ctx context.Context, result productinfo.Output) (*domain.State, error) {
			if result.AwaitingUser {
				return applySubgraphSlotWait(ctx, b, result.MissingSlots, result.FinalAnswer)
			}
			var current *domain.State
			if err := domain.ProcessState(ctx, func(state *domain.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Session.AwaitingUser = false
				state.Session.MissingSlots = nil
				if result.CacheHit {
					state.Response = result.Response
					state.Session.CacheHitLevel = result.HitLevel
					state.Session.FinalAnswer = result.FinalAnswer
					if result.Response != nil {
						state.Session.Intent = result.Response.Intent
					}
					state.Session.Route = domain.RouteProductInfo
					return nil
				}
				state.Session.FinalAnswer = result.FinalAnswer
				state.Session.NeedHandoff = result.NeedHandoff
				state.Session.HandoffReason = result.HandoffReason
				state.Session.ReadOnly = result.ReadOnly
				support.HydrateToolResults(state)
				support.ResetToolState(state)
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
		func(_ context.Context, state *domain.State) (returnexchange.GraphInput, error) {
			return returnexchange.InputFromState(state)
		}),
		compose.WithNodeName("PrepareReturnExchangeInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyReturnExchangeResultNode", compose.InvokableLambda(
		func(ctx context.Context, result returnexchange.Output) (*domain.State, error) {
			if result.AwaitingUser {
				return applySubgraphSlotWait(ctx, b, result.MissingSlots, result.FinalAnswer)
			}
			var current *domain.State
			if err := domain.ProcessState(ctx, func(state *domain.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Session.FinalAnswer = result.FinalAnswer
				state.Session.NeedHandoff = result.NeedHandoff
				state.Session.HandoffReason = result.HandoffReason
				state.Session.ReadOnly = result.ReadOnly
				state.Session.AwaitingConfirm = result.AwaitingConfirm
				state.Session.AwaitingUser = false
				state.Session.MissingSlots = nil
				support.HydrateToolResults(state)
				support.ResetToolState(state)
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
				current.Interrupt = &domain.InterruptState{
					Payload: map[string]any{"confirm": true, "message": reply},
				}
				return current, nil
			}
			return current, nil
		}), compose.WithNodeName("ApplyReturnExchangeResultNode")); err != nil {
		return err
	}

	if err := g.AddLambdaNode("PrepareBaseQAInputNode", compose.InvokableLambda(
		func(_ context.Context, state *domain.State) (baseqa.GraphInput, error) {
			return baseqa.InputFromState(state, b.Skills)
		}),
		compose.WithNodeName("PrepareBaseQAInputNode")); err != nil {
		return err
	}
	if err := g.AddLambdaNode("ApplyBaseQAResultNode", compose.InvokableLambda(
		func(ctx context.Context, result baseqa.Output) (*domain.State, error) {
			var current *domain.State
			if err := domain.ProcessState(ctx, func(state *domain.State) error {
				if state == nil {
					return fmt.Errorf("state is nil")
				}
				current = state
				state.Session.AwaitingUser = false
				state.Session.MissingSlots = nil
				if result.CacheHit {
					state.Response = result.Response
					state.Session.CacheHitLevel = result.HitLevel
					state.Session.FinalAnswer = result.FinalAnswer
					if result.Response != nil {
						state.Session.Intent = result.Response.Intent
					}
					state.Session.Route = domain.RouteBaseQA
					return nil
				}
				state.Session.FinalAnswer = result.FinalAnswer
				state.Rewrite.Query = result.Query
				state.Rewrite.Reason = ""
				state.Retrieval.Documents = append([]*schema.Document(nil), result.Documents...)
				state.Answer.CacheableHint = boolPtr(len(result.Documents) > 0)
				return nil
			}); err != nil {
				return nil, err
			}
			return current, nil
		}), compose.WithNodeName("ApplyBaseQAResultNode")); err != nil {
		return err
	}

	return nil
}
