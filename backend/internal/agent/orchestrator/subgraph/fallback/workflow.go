package fallback

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	fallbacknode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/fallback"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

func fallbackInit(l1 *globalnode.L1SemanticCacheNode) func(context.Context, GraphInput) (GraphInput, error) {
	return func(ctx context.Context, w GraphInput) (GraphInput, error) {
		if l1 == nil {
			return w, nil
		}
		policy := globalnode.ResolveSemanticCachePolicy(domain.RouteBaseQA, w.RawQuery)
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

func branchAfterFallbackL1(_ context.Context, in GraphInput) (string, error) {
	if in.CacheHit {
		return "FallbackL1OutputNode", nil
	}
	return "FallbackRAGNode", nil
}

func buildFallbackL1Output(_ context.Context, in GraphInput) (Output, error) {
	return Output{
		CacheHit:    true,
		HitLevel:    in.HitLevel,
		Response:    in.Response,
		FinalAnswer: in.L1Final,
	}, nil
}

func fallbackRAG(rag *ragnode.RAGNode) func(context.Context, GraphInput) (GraphInput, error) {
	return func(ctx context.Context, in GraphInput) (GraphInput, error) {
		if rag == nil {
			return in, nil
		}
		ragResult, err := rag.Invoke(ctx, ragnode.Input{
			Message: in.RawQuery,
			History: append([]*schema.Message(nil), in.History...),
			Intent:  in.Intent,
		})
		if err != nil {
			return GraphInput{}, err
		}
		if ragResult != nil {
			in.Query = ragResult.Query
			in.Documents = append([]*schema.Document(nil), ragResult.Documents...)
		}
		return in, nil
	}
}

func fallbackModelAgent(agent *sharednode.SubgraphAgent) func(context.Context, GraphInput) (GraphInput, error) {
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
			SystemHint:    prompt.SubgraphSystemFallback,
		})
		if runErr != nil {
			return in, nil
		}
		in.AgentFinal = strings.TrimSpace(final)
		return in, nil
	}
}

func branchAfterFallbackAgent(_ context.Context, in GraphInput) (string, error) {
	if strings.TrimSpace(in.AgentFinal) != "" {
		return "FallbackAgentOutputNode", nil
	}
	return "FallbackBaseQANode", nil
}

func buildFallbackAgentOutput(_ context.Context, in GraphInput) (Output, error) {
	return Output{
		FinalAnswer: in.AgentFinal,
		Query:       in.Query,
		Documents:   append([]*schema.Document(nil), in.Documents...),
	}, nil
}

func fallbackBaseQA(base *fallbacknode.BaseQANode) func(context.Context, GraphInput) (Output, error) {
	return func(ctx context.Context, in GraphInput) (Output, error) {
		out := Output{
			FinalAnswer: in.SeedAnswer,
			Query:       in.Query,
			Documents:   append([]*schema.Document(nil), in.Documents...),
		}
		if base == nil {
			return out, nil
		}
		result, err := base.Invoke(ctx, fallbacknode.BaseQAInput{
			FinalAnswer: out.FinalAnswer,
			Documents:   append([]*schema.Document(nil), out.Documents...),
		})
		if err != nil {
			return Output{}, err
		}
		if result != nil {
			out.FinalAnswer = result.FinalAnswer
		}
		return out, nil
	}
}
