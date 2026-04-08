package returnpolicy

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

type Input struct {
	TenantID     string
	UserID       int64
	SessionID    string
	TraceID      string
	CheckpointID string
	RawQuery     string
	History      []*schema.Message
	Intent       string
}

type Output struct {
	CacheHit    bool
	HitLevel    string
	Response    *domain.ChatResult
	FinalAnswer string
	Query       string
	Documents   []*schema.Document
}

func Build(_ context.Context, ragNode *ragnode.RAGNode, l1Cache *globalnode.L1SemanticCacheNode) (compose.AnyGraph, error) {
	if ragNode == nil && l1Cache == nil {
		return nil, nil
	}

	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ExecuteReturnPolicyFlowNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			policy := globalnode.ResolveSemanticCachePolicy(orchestratorstate.RouteReturnPolicy, input.RawQuery)
			if policy.AllowRead && l1Cache != nil {
				cacheResult, cacheErr := l1Cache.Invoke(ctx, globalnode.L1SemanticCacheInput{
					TenantID:     input.TenantID,
					UserID:       input.UserID,
					Query:        input.RawQuery,
					SessionID:    input.SessionID,
					TraceID:      input.TraceID,
					CheckpointID: input.CheckpointID,
					IntentBucket: policy.IntentBucket,
					Scope:        policy.Scope,
					AllowRead:    true,
				})
				if cacheErr != nil {
					return Output{}, cacheErr
				}
				if cacheResult != nil && cacheResult.CacheHit {
					return Output{
						CacheHit:    true,
						HitLevel:    cacheResult.HitLevel,
						Response:    cacheResult.Response,
						FinalAnswer: cacheResult.FinalAnswer,
					}, nil
				}
			}

			out := Output{}
			if ragNode == nil {
				return out, nil
			}

			ragResult, ragErr := ragNode.Invoke(ctx, ragnode.Input{
				Message: input.RawQuery,
				History: append([]*schema.Message(nil), input.History...),
				Intent:  input.Intent,
			})
			if ragErr != nil {
				return Output{}, ragErr
			}
			if ragResult != nil {
				out.Query = ragResult.Query
				out.Documents = append([]*schema.Document(nil), ragResult.Documents...)
			}
			return out, nil
		}), compose.WithNodeName("ExecuteReturnPolicyFlowNode")); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "ExecuteReturnPolicyFlowNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ExecuteReturnPolicyFlowNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}
