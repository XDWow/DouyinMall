package returnpolicy

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

// rpWire L1 / RAG / 模型阶段共用载体。
type rpWire struct {
	TenantID     string
	UserID       int64
	SessionID    string
	TraceID      string
	CheckpointID string
	RawQuery     string
	History      []*schema.Message
	Intent       string
	SkillNames   []string

	CacheHit   bool
	HitLevel   string
	Response   *domain.ChatResult
	L1Final    string
	Query      string
	Documents  []*schema.Document
	AgentFinal string
}

func loadReturnPolicyWire(ctx context.Context, skills *agentskill.Registry) (rpWire, error) {
	var w rpWire
	err := domain.ProcessState(ctx, func(s *domain.State) error {
		if s == nil {
			return fmt.Errorf("state is nil")
		}
		w.TenantID = s.Session.TenantID
		w.UserID = s.Input.UserID
		w.SessionID = s.Session.SessionID
		w.TraceID = s.TraceID
		w.CheckpointID = s.Checkpoint
		w.RawQuery = s.Session.RawQuery
		w.History = append([]*schema.Message(nil), s.Session.Messages...)
		w.Intent = string(s.Session.Intent)
		w.SkillNames = subgraphmeta.FilteredSkillNames(s.Session.Route, skills)
		return nil
	})
	return w, err
}

func returnPolicyL1Try(l1 *globalnode.L1SemanticCacheNode, skills *agentskill.Registry) func(context.Context, struct{}) (rpWire, error) {
	return func(ctx context.Context, _ struct{}) (rpWire, error) {
		w, err := loadReturnPolicyWire(ctx, skills)
		if err != nil {
			return rpWire{}, err
		}
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
			return rpWire{}, cacheErr
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

func branchAfterReturnPolicyL1(_ context.Context, in rpWire) (string, error) {
	if in.CacheHit {
		return "ReturnPolicyL1OutputNode", nil
	}
	return "ReturnPolicyRAGNode", nil
}

func buildReturnPolicyL1Output(_ context.Context, in rpWire) (Output, error) {
	return Output{
		CacheHit:    true,
		HitLevel:    in.HitLevel,
		Response:    in.Response,
		FinalAnswer: in.L1Final,
	}, nil
}

func returnPolicyRAG(rag *ragnode.RAGNode) func(context.Context, rpWire) (rpWire, error) {
	return func(ctx context.Context, in rpWire) (rpWire, error) {
		if rag == nil {
			return in, nil
		}
		ragResult, ragErr := rag.Invoke(ctx, ragnode.Input{
			Message: in.RawQuery,
			History: append([]*schema.Message(nil), in.History...),
			Intent:  in.Intent,
		})
		if ragErr != nil {
			return rpWire{}, ragErr
		}
		if ragResult != nil {
			in.Query = ragResult.Query
			in.Documents = append([]*schema.Document(nil), ragResult.Documents...)
		}
		return in, nil
	}
}

func returnPolicyModelAgent(agent *sharednode.SubgraphAgent) func(context.Context, rpWire) (rpWire, error) {
	return func(ctx context.Context, in rpWire) (rpWire, error) {
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

func buildReturnPolicyFinalOutput(_ context.Context, in rpWire) (Output, error) {
	out := Output{
		Query:       in.Query,
		Documents:   append([]*schema.Document(nil), in.Documents...),
		FinalAnswer: in.AgentFinal,
	}
	return out, nil
}
