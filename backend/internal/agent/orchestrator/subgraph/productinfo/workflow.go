package productinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	productnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/product"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
	productinfometa "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productinfo/metadata"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

// piWire 商品子图内流水线载体（首节点从 State 快照填充）。
type piWire struct {
	TenantID     string
	UserID       int64
	SessionID    string
	TraceID      string
	CheckpointID string
	RawQuery     string
	History      []*schema.Message
	Intent       string
	SkillNames   []string
	IntentFields map[string]string // 本回合 s.Intent.Entities 的拷贝，用于拼工具入参

	CacheHit     bool
	HitLevel     string
	Response     *domain.ChatResult
	L1Final      string
	Slots        map[string]any
	MissingSlots []string
	Documents    []*schema.Document
	DocsText     string
	AgentFinal   string
	AgentTools   []*schema.Message
}

func loadProductInfoWire(ctx context.Context, skills *agentskill.Registry) (piWire, error) {
	var w piWire
	err := domain.ProcessState(ctx, func(s *domain.State) error {
		if s == nil {
			return fmt.Errorf("state is nil")
		}
		w.TenantID = s.Session.TenantID
		w.UserID = s.Input.UserID
		w.SessionID = s.Session.SessionID
		w.TraceID = s.TraceID
		w.CheckpointID = s.Checkpoint
		w.Slots = cloneSlotsPI(s.Session.Slots)
		w.RawQuery = s.Session.RawQuery
		w.History = append([]*schema.Message(nil), s.Session.Messages...)
		w.Intent = string(s.Session.Intent)
		w.SkillNames = subgraphmeta.FilteredSkillNames(s.Session.Route, skills)
		w.IntentFields = s.Intent.Entities
		return nil
	})
	return w, err
}

func cloneSlotsPI(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func productInfoL1Try(l1 *globalnode.L1SemanticCacheNode, skills *agentskill.Registry) func(context.Context, struct{}) (piWire, error) {
	return func(ctx context.Context, _ struct{}) (piWire, error) {
		w, err := loadProductInfoWire(ctx, skills)
		if err != nil {
			return piWire{}, err
		}
		if l1 == nil {
			return w, nil
		}
		policy := globalnode.ResolveSemanticCachePolicy(domain.RouteProductInfo, w.RawQuery)
		if !policy.AllowRead {
			return w, nil
		}
		res, err := l1.Invoke(ctx, globalnode.L1SemanticCacheInput{
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
		if err != nil {
			return piWire{}, err
		}
		if res != nil && res.CacheHit {
			w.CacheHit = true
			w.HitLevel = res.HitLevel
			w.Response = res.Response
			w.L1Final = res.FinalAnswer
		}
		return w, nil
	}
}

func branchAfterProductL1(_ context.Context, in piWire) (string, error) {
	if in.CacheHit {
		return "ProductInfoL1OutputNode", nil
	}
	return "ProductInfoPrepareSlotsNode", nil
}

func buildProductInfoL1Output(_ context.Context, in piWire) (Output, error) {
	return Output{
		CacheHit:    true,
		HitLevel:    in.HitLevel,
		Response:    in.Response,
		FinalAnswer: in.L1Final,
	}, nil
}

func productInfoPrepareSlots(_ context.Context, in piWire) (piWire, error) {
	slots := cloneSlotsPI(in.Slots)
	if slots == nil {
		slots = map[string]any{}
	}
	globalnode.ApplyIntentFieldsForTools(slots, in.IntentFields)
	missing := globalnode.RequiredMissingSlots(domain.IntentProductInfo, slots, in.IntentFields, false)
	in.Slots = slots
	in.MissingSlots = missing
	return in, nil
}

func branchAfterProductSlotCheck(_ context.Context, in piWire) (string, error) {
	if len(in.MissingSlots) > 0 {
		return "ProductInfoMissingSlotsNode", nil
	}
	return "ProductInfoRAGNode", nil
}

func buildProductInfoMissingOutput(_ context.Context, in piWire) (Output, error) {
	m := in.MissingSlots[0]
	return Output{
		FinalAnswer:  globalnode.AskMessageForMissingSlot(domain.IntentProductInfo, m),
		ReadOnly:     true,
		AwaitingUser: true,
		MissingSlots: append([]string(nil), in.MissingSlots...),
		Query:        in.RawQuery,
	}, nil
}

func productInfoRAG(rag *ragnode.RAGNode) func(context.Context, piWire) (piWire, error) {
	return func(ctx context.Context, in piWire) (piWire, error) {
		if rag == nil || !support.IsAdvisoryProductInfo(in.RawQuery) {
			return in, nil
		}
		ragResult, ragErr := rag.Invoke(ctx, ragnode.Input{
			Message: in.RawQuery,
			History: append([]*schema.Message(nil), in.History...),
			Intent:  in.Intent,
		})
		if ragErr != nil {
			return piWire{}, ragErr
		}
		if ragResult != nil {
			in.Documents = append([]*schema.Document(nil), ragResult.Documents...)
			in.DocsText = support.DocumentsText(ragResult.Documents)
		}
		return in, nil
	}
}

func productInfoModelAgent(agent *sharednode.SubgraphAgent) func(context.Context, piWire) (piWire, error) {
	return func(ctx context.Context, in piWire) (piWire, error) {
		if agent == nil || !agent.Enabled() {
			return in, nil
		}
		slotCtx, _ := json.Marshal(in.Slots)
		final, tmsgs, runErr := agent.Run(ctx, sharednode.SubgraphAgentInput{
			ToolNames:     productinfometa.AllowedToolNames(),
			SkillNames:    append([]string(nil), in.SkillNames...), // 拷贝：本路由允许的技能白名单，与 ToolNames 一起在 SubgraphAgent 里生效
			DocumentsText: in.DocsText,
			SlotsContext:  string(slotCtx),
			UserQuery:     in.RawQuery,
			History:       append([]*schema.Message(nil), in.History...),
			SystemHint:    prompt.SubgraphSystemProductInfo,
		})
		if runErr != nil {
			return in, nil
		}
		in.AgentFinal = strings.TrimSpace(final)
		in.AgentTools = append([]*schema.Message(nil), tmsgs...)
		return in, nil
	}
}

func branchAfterProductAgent(_ context.Context, in piWire) (string, error) {
	if strings.TrimSpace(in.AgentFinal) != "" {
		return "ProductInfoAgentAnswerNode", nil
	}
	return "ProductInfoRulePlanNode", nil
}

func buildProductInfoAgentOutput(ctx context.Context, in piWire) (Output, error) {
	_ = domain.ProcessState(ctx, func(s *domain.State) error {
		if s != nil && s.Recorder != nil {
			support.HydrateToolResultsIntoSlots(in.Slots, s.Recorder.Snapshot())
		}
		return nil
	})
	return Output{
		FinalAnswer:  in.AgentFinal,
		ReadOnly:     true,
		ToolMessages: append([]*schema.Message(nil), in.AgentTools...),
		Query:        in.RawQuery,
		Documents:    append([]*schema.Document(nil), in.Documents...),
	}, nil
}

func productInfoRulePlanAndTools(
	node *productnode.ProductInfoNode,
	toolExec *sharednode.ToolExecNode,
) func(context.Context, piWire) (Output, error) {
	return func(ctx context.Context, in piWire) (Output, error) {
		result, err := node.Invoke(ctx, productnode.ProductInfoInput{
			Slots:    in.Slots,
			RawQuery: in.RawQuery,
		})
		if err != nil {
			return Output{}, err
		}
		out := Output{
			FinalAnswer:   result.FinalAnswer,
			NeedHandoff:   result.NeedHandoff,
			HandoffReason: result.HandoffReason,
			ReadOnly:      result.ReadOnly,
			Query:         in.RawQuery,
			Documents:     append([]*schema.Document(nil), in.Documents...),
		}
		if len(result.Plans) == 0 || toolExec == nil {
			_ = domain.ProcessState(ctx, func(s *domain.State) error {
				if s != nil && s.Recorder != nil {
					support.HydrateToolResultsIntoSlots(in.Slots, s.Recorder.Snapshot())
				}
				return nil
			})
			return out, nil
		}
		callMessage, callErr := toolexec.CreateToolCallMessage(result.Plans)
		if callErr != nil {
			return Output{}, callErr
		}
		messages, execErr := toolExec.Invoke(ctx, sharednode.ToolExecutionInput{
			Plans:       result.Plans,
			CallMessage: callMessage,
			Mode:        agenttool.ToolExecutionParallelReadOnly,
		})
		if execErr != nil {
			return Output{}, execErr
		}
		out.ToolMessages = append([]*schema.Message(nil), messages...)
		_ = domain.ProcessState(ctx, func(s *domain.State) error {
			if s != nil && s.Recorder != nil {
				support.HydrateToolResultsIntoSlots(in.Slots, s.Recorder.Snapshot())
			}
			return nil
		})
		return out, nil
	}
}
