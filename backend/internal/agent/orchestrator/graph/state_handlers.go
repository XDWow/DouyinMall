package graph

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
)

// Eino 状态图约定（与官方 checkpoint-and-state 文档一致）：
//
// 1) StatePreHandler[I,S] / StatePostHandler[O,S] 挂在 AddLambdaNode 外侧，在框架持锁下改写「即将进入 / 刚离开」Lambda 的 I、O。
// 2) Lambda 尽量为恒等传递；从 State 取字段、调用可单测的 Invoke(in)→result、再写回 State 的逻辑放在 Pre/Post。
// 3) 边上传递的 *domain.State 以 in/out 为权威读写目标（与官方「通过改 Input/Output 影响节点」一致）。
// 4) GenLocalState 的 S 与边上指针可能不同；仅输入为子图 Output 的节点需用 ProcessState 读 S。线性段每个 Pre/Post 末尾 syncGenFromEdge(in|out, st)，使 S 与边上一致。
//
// 5) 若节点 Lambda 的 I/O 不是 *domain.State（例如 Apply* 节点为 subgraph.Output→*State），则无法对该节点使用「O 为 *State 的 PostHandler」以外的同一套 Handler 类型对齐；此类节点仍用 Lambda 内 ProcessState 合并（官方推荐的另一种形态）。

func syncGenFromEdge(edge, gen *domain.State) {
	if edge == nil || gen == nil {
		return
	}
	*gen = *edge
}

func statePassthrough() func(ctx context.Context, state *domain.State) (*domain.State, error) {
	return func(ctx context.Context, state *domain.State) (*domain.State, error) {
		return state, nil
	}
}

func accessGuardStatePre(ag *globalnode.AccessGuardNode) compose.StatePreHandler[*domain.State, *domain.State] {
	return func(ctx context.Context, in *domain.State, st *domain.State) (*domain.State, error) {
		if in == nil {
			return nil, fmt.Errorf("state is nil")
		}
		result, err := ag.Invoke(ctx, globalnode.AccessGuardInput{
			Message:     in.Input.Message,
			UserID:      in.Input.UserID,
			ResumeToken: in.Input.ResumeToken,
		})
		if err != nil {
			return in, err
		}
		session := &in.Session
		session.UserID = in.Input.UserID
		session.RawQuery = result.RawQuery
		session.TenantID = result.TenantID
		session.ResumeFromCP = result.ResumeFromCP
		session.ErrorCode = result.ErrorCode
		session.FinalAnswer = result.FinalAnswer
		if result.ErrorCode != "" {
			in.Answer.CacheableHint = boolPtr(false)
		}
		in.Interrupt = nil
		syncGenFromEdge(in, st)
		return in, nil
	}
}

func sessionLoadStatePre(sl *globalnode.SessionLoadNode) compose.StatePreHandler[*domain.State, *domain.State] {
	return func(ctx context.Context, in *domain.State, st *domain.State) (*domain.State, error) {
		if in == nil {
			return nil, fmt.Errorf("state is nil")
		}
		if err := sl.PrepareSession(ctx, in); err != nil {
			return in, err
		}
		syncGenFromEdge(in, st)
		return in, nil
	}
}

func cachePreCheckStatePre(n *globalnode.CachePreCheckNode) compose.StatePreHandler[*domain.State, *domain.State] {
	return func(ctx context.Context, in *domain.State, st *domain.State) (*domain.State, error) {
		if in == nil {
			return nil, fmt.Errorf("state is nil")
		}
		result, err := n.Invoke(ctx, globalnode.CachePreCheckInput{
			TenantID:        in.Session.TenantID,
			UserID:          in.Input.UserID,
			Message:         in.Session.RawQuery,
			ResumeFromCP:    in.Session.ResumeFromCP,
			AwaitingUser:    in.Session.AwaitingUser,
			AwaitingConfirm: in.Session.AwaitingConfirm,
		})
		if err != nil {
			return in, err
		}
		in.Cache.AllowExact = result.AllowExact
		in.Cache.AllowSemantic = result.AllowSemantic
		in.Cache.IntentBucket = result.IntentBucket
		in.Cache.Scope = string(result.Scope)
		syncGenFromEdge(in, st)
		return in, nil
	}
}

func l0ExactCacheStatePre(n *globalnode.L0ExactCacheNode) compose.StatePreHandler[*domain.State, *domain.State] {
	return func(ctx context.Context, in *domain.State, st *domain.State) (*domain.State, error) {
		if in == nil {
			return nil, fmt.Errorf("state is nil")
		}
		result, err := n.Invoke(ctx, globalnode.L0ExactCacheInput{
			TenantID:     in.Session.TenantID,
			UserID:       in.Input.UserID,
			RawQuery:     in.Session.RawQuery,
			SessionID:    in.Input.SessionID,
			TraceID:      in.TraceID,
			CheckpointID: in.Checkpoint,
			AllowRead:    in.Cache.AllowExact,
		})
		if err != nil {
			return in, err
		}
		if result.CacheHit {
			in.Response = result.Response
			in.Session.CacheHitLevel = result.HitLevel
		}
		syncGenFromEdge(in, st)
		return in, nil
	}
}

func intentAndSlotStatePre(intent *globalnode.IntentAndSlotNode, merge *globalnode.SlotMergeNode) compose.StatePreHandler[*domain.State, *domain.State] {
	return func(ctx context.Context, in *domain.State, st *domain.State) (*domain.State, error) {
		if in == nil {
			return nil, fmt.Errorf("state is nil")
		}
		out, err := intent.Invoke(ctx, in)
		if err != nil {
			return in, err
		}
		if out == nil {
			out = in
		}
		if merge != nil {
			merged, mergeErr := merge.Apply(ctx, out)
			if mergeErr != nil {
				return out, mergeErr
			}
			if merged != nil {
				out = merged
			}
		}
		syncGenFromEdge(out, st)
		return out, nil
	}
}

func routeStatePre(n *globalnode.RouteNode) compose.StatePreHandler[*domain.State, *domain.State] {
	return func(ctx context.Context, in *domain.State, st *domain.State) (*domain.State, error) {
		if in == nil {
			return nil, fmt.Errorf("state is nil")
		}
		result, err := n.Invoke(ctx, globalnode.RouteInput{
			Intent:          in.Intent.Intent,
			AwaitingConfirm: in.Session.AwaitingConfirm,
		})
		if err != nil {
			return in, err
		}
		session := &in.Session
		session.Route = result.Route
		session.ErrorCode = result.ErrorCode
		session.ReadOnly = result.ReadOnly
		syncGenFromEdge(in, st)
		return in, nil
	}
}

func finalizeStatePost(fz *globalnode.FinalizeNode) compose.StatePostHandler[*domain.State, *domain.State] {
	return func(ctx context.Context, out *domain.State, st *domain.State) (*domain.State, error) {
		if out == nil {
			return nil, fmt.Errorf("state is nil")
		}
		if err := fz.FinalizeSession(ctx, out); err != nil {
			return out, err
		}
		syncGenFromEdge(out, st)
		return out, nil
	}
}

// BranchFromAccessGuard 基于分支 API 传入的 in（前驱节点输出），不通过 ProcessState 取 State。
func BranchFromAccessGuard(_ context.Context, in *domain.State) (string, error) {
	if in != nil && in.Session.ErrorCode != "" {
		return "FinalizeNode", nil
	}
	return "SessionLoadNode", nil
}

func BranchAfterL0Cache(_ context.Context, in *domain.State) (string, error) {
	if in != nil && in.Session.CacheHitLevel != "" {
		return "FinalizeNode", nil
	}
	return "IntentAndSlotNode", nil
}

func BranchFromRoute(_ context.Context, in *domain.State) (string, error) {
	if in == nil {
		return "PrepareBaseQAInputNode", nil
	}
	switch in.Session.Route {
	case domain.RouteOrderQuery:
		return "PrepareOrderQueryInputNode", nil
	case domain.RouteInventory:
		return "PrepareInventoryInputNode", nil
	case domain.RouteProductInfo:
		return "PrepareProductInfoInputNode", nil
	case domain.RouteAddToCart:
		return "PrepareAddToCartInputNode", nil
	case domain.RouteReturnPolicy:
		return "PrepareReturnPolicyInputNode", nil
	case domain.RouteReturnExchangeApply:
		return "PrepareReturnExchangeInputNode", nil
	default:
		return "PrepareBaseQAInputNode", nil
	}
}

func BranchAfterFinalize(_ context.Context, in *domain.State) (string, error) {
	if in != nil && in.Interrupt != nil && len(in.Interrupt.Payload) > 0 {
		return "InterruptNode", nil
	}
	return compose.END, nil
}
