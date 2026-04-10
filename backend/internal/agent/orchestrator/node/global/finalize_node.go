package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

const lowConfidenceThreshold = 0.65

// FinalizeNode 主图出口：生产路径由 graph 包注册的 StatePostHandler 调用 FinalizeSession（框架已持 state 锁）。
type FinalizeNode struct {
	SessionService *agentsession.Service
	CacheWriteback *CacheWritebackService
	Logger         logger.LoggerV1
	Metrics        *orchestratorobserve.Metrics
}

func NewFinalizeNode(
	sessionService *agentsession.Service,
	cacheWriteback *CacheWritebackService,
	log logger.LoggerV1,
	metrics *orchestratorobserve.Metrics,
) *FinalizeNode {
	return &FinalizeNode{
		SessionService: sessionService,
		CacheWriteback: cacheWriteback,
		Logger:         log,
		Metrics:        metrics,
	}
}

// FinalizeSession 汇总回复、流式输出、落库与缓存；与图编排解耦，可直接单测。
func (n *FinalizeNode) FinalizeSession(ctx context.Context, st *domain.State) error {
	if st == nil {
		return fmt.Errorf("state is required")
	}

	in := st.Input
	resp := domain.EnsureChatResult(in, st)
	resp.SessionID = support.FirstNonEmpty(resp.SessionID, st.Session.SessionID, in.SessionID)

	reply, source := n.resolveReply(st, resp)
	reply = support.NormalizeReply(reply)

	confidence := resp.Confidence
	if confidence <= 0 {
		confidence = support.EstimateConfidence(st)
	}
	reply = n.decorateLowConfidenceReply(reply, confidence, st)

	usedToolNames := support.CollectUsedToolNames(st)
	st.Answer = domain.AnswerResult{
		Reply:         reply,
		Confidence:    confidence,
		Source:        source,
		CacheableHint: st.Answer.CacheableHint,
		Streamed:      st.Answer.Streamed,
		UsedToolNames: append([]string(nil), usedToolNames...),
	}

	resp.Reply = reply
	if resp.Intent == domain.IntentUnknown {
		resp.Intent = st.Intent.Intent
	}
	if resp.Intent == domain.IntentUnknown {
		resp.Intent = st.Session.Intent
	}
	resp.Confidence = confidence
	if len(resp.References) == 0 {
		resp.References = support.DocumentsToReferences(st.Retrieval.Documents)
	}
	resp.ToolExecutions = nil
	resp.UsedToolNames = append([]string(nil), usedToolNames...)
	if !resp.NeedHandoff {
		resp.NeedHandoff = st.Intent.NeedHandoff
	}
	if !resp.NeedHandoff {
		resp.NeedHandoff = st.Answer.NeedHandoff
	}
	if strings.TrimSpace(resp.HandoffReason) == "" {
		resp.HandoffReason = st.Answer.HandoffReason
	}
	resp.Trace.TraceID = st.TraceID
	resp.Trace.CheckpointID = st.Checkpoint
	resp.Trace.CacheHit = st.Session.CacheHitLevel != ""
	resp.Trace.RewrittenQuery = st.Rewrite.Query

	if resp.NeedHandoff {
		resp.Status = domain.ReplyStatusHandoff
		if n.Metrics != nil {
			n.Metrics.ObserveHandoff(resp.HandoffReason)
		}
	} else {
		resp.Status = domain.ReplyStatusAnswered
	}

	if err := n.emitReply(ctx, st, reply); err != nil {
		return err
	}
	support.ResetToolState(st)
	n.persistTurn(ctx, st, resp)
	return nil
}

func (n *FinalizeNode) resolveReply(st *domain.State, resp *domain.ChatResult) (string, string) {
	if resp != nil && strings.TrimSpace(resp.Reply) != "" {
		if st.Session.CacheHitLevel != "" {
			return resp.Reply, "cache"
		}
		return resp.Reply, "response"
	}
	if strings.TrimSpace(st.Session.FinalAnswer) != "" {
		return st.Session.FinalAnswer, "subgraph"
	}
	return support.TemplateAnswer(st), "template"
}

func (n *FinalizeNode) decorateLowConfidenceReply(reply string, confidence float64, st *domain.State) string {
	if strings.TrimSpace(reply) == "" || confidence >= lowConfidenceThreshold {
		return reply
	}
	if st != nil && st.Answer.Streamed {
		return reply
	}
	if st != nil && (st.Session.NeedHandoff || st.Session.AwaitingUser || st.Session.AwaitingConfirm) {
		return reply
	}
	notice := "置信度偏低，请补充关键信息。"
	if strings.Contains(reply, notice) {
		return reply
	}
	return strings.TrimSpace(reply + "\n\n" + notice)
}

func (n *FinalizeNode) emitReply(ctx context.Context, st *domain.State, reply string) error {
	if st == nil || st.StreamWriter == nil || st.Answer.Streamed || strings.TrimSpace(reply) == "" {
		return nil
	}
	orchestratorobserve.SendEvent(ctx, st.StreamWriter, "token", map[string]any{
		"trace_id": st.TraceID,
		"text":     reply,
	})
	st.Answer.Streamed = true
	return nil
}

func (n *FinalizeNode) persistTurn(ctx context.Context, state *domain.State, resp *domain.ChatResult) {
	if state == nil || resp == nil || state.PersistedSession == nil {
		return
	}

	userMsg := agentsession.NewUserMessage(state.PersistedSession.SessionID, state.Session.RawQuery)
	assistantMsg := agentsession.NewAssistantMessage(state.PersistedSession.SessionID, resp.Reply, resp.Intent, resp.Confidence)
	if strings.TrimSpace(userMsg.Content) == "" && strings.TrimSpace(assistantMsg.Content) == "" {
		return
	}

	persistedSnapshot := *state.PersistedSession
	persistedSnapshot.Slots = mergePersistedSessionState(state.Session.Slots, state.Session.CurrentRefs, state.Session.PendingSelections)
	persistedSnapshot.TotalTurns++
	persistedSnapshot.UpdatedAt = assistantMsg.CreatedAt
	if persistedSnapshot.UpdatedAt.IsZero() {
		persistedSnapshot.UpdatedAt = time.Now()
	}
	if strings.TrimSpace(assistantMsg.Content) != "" {
		persistedSnapshot.LastMessage = agentsession.Truncate(assistantMsg.Content, 64)
	} else {
		persistedSnapshot.LastMessage = agentsession.Truncate(userMsg.Content, 64)
	}
	if persistedSnapshot.Status == "" {
		persistedSnapshot.Status = domain.SessionStatusActive
	}
	state.PersistedSession = &persistedSnapshot
	state.Session.ApplyPersistedFields(persistedSnapshot)

	if n.SessionService != nil {
		history := agentsession.FromSchemaMessages(persistedSnapshot.SessionID, append([]*schema.Message(nil), state.Session.Messages...))
		if strings.TrimSpace(userMsg.Content) != "" {
			history = append(history, userMsg)
		}
		if strings.TrimSpace(assistantMsg.Content) != "" {
			history = append(history, assistantMsg)
		}
		if err := n.SessionService.SaveTurnCache(ctx, persistedSnapshot, history); err != nil && n.Logger != nil {
			n.Logger.Warn("会话缓存失败", logger.Error(err))
		}
	}

	if n.SessionService == nil && n.CacheWriteback == nil {
		return
	}

	go n.persistInBackground(context.WithoutCancel(ctx), persistedSnapshot, userMsg, assistantMsg, cloneStateForAsyncPersist(state))
}

func (n *FinalizeNode) persistInBackground(
	ctx context.Context,
	session domain.Session,
	userMsg domain.SessionMessage,
	assistantMsg domain.SessionMessage,
	state *domain.State,
) {
	bgCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if n.SessionService != nil {
		if err := n.SessionService.SaveTurnPersistent(bgCtx, session, userMsg, assistantMsg); err != nil && n.Logger != nil {
			n.Logger.Warn("会话落库失败", logger.Error(err))
		}
	}
	if n.CacheWriteback != nil {
		if err := n.CacheWriteback.Write(bgCtx, state); err != nil && n.Logger != nil {
			n.Logger.Warn("答案缓存失败", logger.Error(err))
		}
	}
}

func cloneStateForAsyncPersist(state *domain.State) *domain.State {
	if state == nil {
		return nil
	}

	cloned := *state
	cloned.StreamWriter = nil
	cloned.Recorder = nil
	cloned.Session = state.Session
	cloned.Session.Slots = cloneSessionSlots(state.Session.Slots)
	cloned.Session.Messages = append([]*schema.Message(nil), state.Session.Messages...)
	cloned.Session.PendingSelections = clonePendingSelections(state.Session.PendingSelections)
	cloned.Rewrite = state.Rewrite
	cloned.Retrieval = domain.RetrievalResult{
		Documents: append([]*schema.Document(nil), state.Retrieval.Documents...),
	}
	cloned.Tool = domain.ToolState{}
	cloned.Answer = state.Answer
	if state.Interrupt != nil {
		cloned.Interrupt = &domain.InterruptState{
			Payload: cloneSessionSlots(state.Interrupt.Payload),
		}
	}
	if state.Response != nil {
		resp := *state.Response
		resp.References = append([]domain.KnowledgeRef(nil), state.Response.References...)
		resp.ToolExecutions = nil
		resp.UsedToolNames = append([]string(nil), state.Response.UsedToolNames...)
		resp.Trace.Steps = append([]domain.TraceStep(nil), state.Response.Trace.Steps...)
		cloned.Response = &resp
	}
	if state.PersistedSession != nil {
		persistedSession := *state.PersistedSession
		persistedSession.Slots = cloneSessionSlots(state.PersistedSession.Slots)
		cloned.PersistedSession = &persistedSession
	}
	return &cloned
}

func cloneSessionSlots(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
