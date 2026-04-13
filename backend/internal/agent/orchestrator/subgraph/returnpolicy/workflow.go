package returnpolicy

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

func returnPolicyL1Try(l1 *globalnode.L1SemanticCacheNode) func(context.Context, GraphInput) (GraphInput, error) {
	return func(ctx context.Context, w GraphInput) (GraphInput, error) {
		if l1 == nil {
			return w, nil
		}
		policy := globalnode.ResolveSemanticCachePolicy(domain.RouteReturnPolicy, w.RawQuery)
		if !policy.AllowRead {
			return w, nil
		}
		cacheResult, cacheErr := l1.Invoke(ctx, globalnode.L1SemanticCacheInput{
			TenantID:     w.TenantID,
			UserID:       w.UserID,
			Query:        w.RawQuery,
			SessionID:    w.SessionID,
			TraceID:      w.TraceID,
			CheckpointID: w.CheckpointID,
			IntentBucket: policy.IntentBucket,
			Scope:        policy.Scope,
			AllowRead:    true,
		})
		if cacheErr != nil {
			return GraphInput{}, cacheErr
		}
		if cacheResult != nil && cacheResult.CacheHit {
			w.CacheHit = true
			w.HitLevel = cacheResult.HitLevel
			w.Response = cacheResult.Response
			w.L1Final = cacheResult.FinalAnswer
		}
		return w, nil
	}
}

func branchAfterReturnPolicyL1(_ context.Context, in GraphInput) (string, error) {
	if in.CacheHit {
		return "ReturnPolicyL1OutputNode", nil
	}
	return "ReturnPolicyRAGNode", nil
}

func buildReturnPolicyL1Output(_ context.Context, in GraphInput) (Output, error) {
	return Output{
		CacheHit:    true,
		HitLevel:    in.HitLevel,
		Response:    in.Response,
		FinalAnswer: in.L1Final,
	}, nil
}

func returnPolicyRAG(rag *ragnode.RAGNode) func(context.Context, GraphInput) (GraphInput, error) {
	return func(ctx context.Context, in GraphInput) (GraphInput, error) {
		if rag == nil {
			return in, nil
		}
		ragResult, ragErr := rag.Invoke(ctx, ragnode.Input{
			Message: in.RawQuery,
			History: append([]*schema.Message(nil), in.History...),
			Intent:  in.Intent,
		})
		if ragErr != nil {
			return GraphInput{}, ragErr
		}
		if ragResult != nil {
			in.Query = ragResult.Query
			in.Documents = append([]*schema.Document(nil), ragResult.Documents...)
		}
		return in, nil
	}
}

func returnPolicyModelAgent(agent *sharednode.SubgraphAgent) func(context.Context, GraphInput) (GraphInput, error) {
	return func(ctx context.Context, in GraphInput) (GraphInput, error) {
		if agent == nil || !agent.Enabled() {
			return in, nil
		}
		docs := support.DocumentsText(in.Documents)
		final, _, runErr := agent.Run(ctx, sharednode.SubgraphAgentInput{
			SkillNames:    append([]string(nil), in.SkillNames...),
			DocumentsText: docs,
			UserQuery:     in.RawQuery,
			History:       append([]*schema.Message(nil), in.History...),
			SystemHint:    prompt.SubgraphSystemReturnPolicy,
		})
		if runErr != nil {
			return in, nil
		}
		in.AgentFinal = strings.TrimSpace(final)
		return in, nil
	}
}

func buildReturnPolicyFinalOutput(_ context.Context, in GraphInput) (Output, error) {
	out := Output{
		Query:       in.Query,
		Documents:   append([]*schema.Document(nil), in.Documents...),
		FinalAnswer: in.AgentFinal,
	}
	return out, nil
}
