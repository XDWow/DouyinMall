package global

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

const lowConfidenceThreshold = 0.65

type FinalizeNode struct {
	Model          model.ToolCallingChatModel
	Prompts        *orchestratorprompt.Set
	Skills         *agentskill.Registry
	Tools          *agenttool.Registry
	SessionService *agentsession.Service
	CacheWriteback *CacheWritebackService
	Logger         logger.LoggerV1
	Metrics        *orchestratorobserve.Metrics
	MaxTokens      int
}

func NewFinalizeNode(
	chatModel model.ToolCallingChatModel,
	prompts *orchestratorprompt.Set,
	skills *agentskill.Registry,
	tools *agenttool.Registry,
	sessionService *agentsession.Service,
	cacheWriteback *CacheWritebackService,
	log logger.LoggerV1,
	metrics *orchestratorobserve.Metrics,
	maxTokens int,
) *FinalizeNode {
	return &FinalizeNode{
		Model:          chatModel,
		Prompts:        prompts,
		Skills:         skills,
		Tools:          tools,
		SessionService: sessionService,
		CacheWriteback: cacheWriteback,
		Logger:         log,
		Metrics:        metrics,
		MaxTokens:      maxTokens,
	}
}

func (n *FinalizeNode) Invoke(ctx context.Context, state *graphstate.State) (*graphstate.State, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}

	resp := state.EnsureResponse()
	resp.SessionID = support.FirstNonEmpty(resp.SessionID, state.Session.SessionID, state.Request.SessionID)

	if err := n.generateReplyIfNeeded(ctx, state); err != nil {
		return nil, err
	}

	reply, source := n.resolveReply(state, resp)
	reply = support.NormalizeReply(reply)

	confidence := resp.Confidence
	if confidence <= 0 {
		confidence = support.EstimateConfidence(state)
	}
	reply = n.decorateLowConfidenceReply(reply, confidence, state)

	state.Answer = graphstate.AnswerResult{
		Reply:         reply,
		Confidence:    confidence,
		Source:        source,
		CacheableHint: state.Answer.CacheableHint,
		Streamed:      state.Answer.Streamed,
	}

	resp.Reply = reply
	if resp.Intent == domain.IntentUnknown {
		resp.Intent = state.Session.Intent
	}
	resp.Confidence = confidence
	if len(resp.References) == 0 {
		resp.References = support.DocumentsToReferences(state.Retrieval.Documents)
	}
	if len(resp.ToolExecutions) == 0 {
		resp.ToolExecutions = state.ToolExecutions()
	}
	if !resp.NeedHandoff {
		resp.NeedHandoff = state.Session.NeedHandoff
	}
	if strings.TrimSpace(resp.HandoffReason) == "" {
		resp.HandoffReason = state.Session.HandoffReason
	}
	resp.Trace.TraceID = state.TraceID
	resp.Trace.CheckpointID = state.Checkpoint
	resp.Trace.CacheHit = state.Session.CacheHitLevel != ""
	resp.Trace.RewrittenQuery = state.Rewrite.Query

	if resp.NeedHandoff {
		resp.Status = domain.ReplyStatusHandoff
		if n.Metrics != nil {
			n.Metrics.ObserveHandoff(resp.HandoffReason)
		}
	} else {
		resp.Status = domain.ReplyStatusAnswered
	}

	if err := n.emitReply(ctx, state, reply); err != nil {
		return nil, err
	}
	n.persistTurn(ctx, state, resp)
	return state, nil
}

func (n *FinalizeNode) generateReplyIfNeeded(ctx context.Context, state *graphstate.State) error {
	if state == nil || n == nil {
		return nil
	}
	if strings.TrimSpace(state.EnsureResponse().Reply) != "" || strings.TrimSpace(state.Session.FinalAnswer) != "" {
		return nil
	}
	if !support.ShouldUseLLMAnswer(state) || n.Model == nil || n.Prompts == nil || n.Prompts.Answer == nil {
		return nil
	}

	messages, err := n.Prompts.Answer.Format(ctx, map[string]any{
		"system_text":           n.Prompts.SystemText,
		"history":               append([]*schema.Message(nil), state.Session.Messages...),
		"message":               state.Session.RawQuery,
		"query":                 support.FirstNonEmpty(state.Rewrite.Query, state.Session.RawQuery),
		"documents_text":        support.DocumentsText(state.Retrieval.Documents),
		"tool_text":             support.ToolText(state.ToolExecutions()),
		"tool_definitions_text": n.selectedToolText(state),
		"skill_text":            n.selectedSkillText(state.Skill.Names),
	})
	if err != nil {
		if n.Logger != nil {
			n.Logger.Warn("format final answer prompt failed", logger.Error(err))
		}
		return nil
	}

	reply, err := n.generate(ctx, state, messages)
	if err != nil {
		return err
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}

	state.Session.FinalAnswer = reply
	state.Answer.Reply = reply
	state.Answer.Source = "llm_stream"
	return nil
}

func (n *FinalizeNode) generate(ctx context.Context, state *graphstate.State, messages []*schema.Message) (string, error) {
	options := []model.Option{
		model.WithTemperature(0.15),
		model.WithToolChoice(schema.ToolChoiceForbidden),
	}
	if n.MaxTokens > 0 {
		options = append(options, model.WithMaxTokens(n.MaxTokens))
	}

	if state.StreamWriter == nil {
		msg, err := n.Model.Generate(ctx, messages, options...)
		if err != nil || msg == nil {
			return "", err
		}
		return msg.Content, nil
	}

	stream, err := n.Model.Stream(ctx, messages, options...)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var builder strings.Builder
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return "", recvErr
		}
		if chunk == nil || strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		builder.WriteString(chunk.Content)
		orchestratorobserve.SendEvent(ctx, state.StreamWriter, "token", map[string]any{
			"trace_id": state.TraceID,
			"text":     chunk.Content,
		})
	}
	state.Answer.Streamed = builder.Len() > 0
	return builder.String(), nil
}

func (n *FinalizeNode) resolveReply(state *graphstate.State, resp *domain.ChatResult) (string, string) {
	if resp != nil && strings.TrimSpace(resp.Reply) != "" {
		if state.Session.CacheHitLevel != "" {
			return resp.Reply, "cache"
		}
		return resp.Reply, "response"
	}
	if strings.TrimSpace(state.Session.FinalAnswer) != "" {
		return state.Session.FinalAnswer, "subgraph"
	}
	return support.TemplateAnswer(state), "template"
}

func (n *FinalizeNode) decorateLowConfidenceReply(reply string, confidence float64, state *graphstate.State) string {
	if strings.TrimSpace(reply) == "" || confidence >= lowConfidenceThreshold {
		return reply
	}
	if state != nil && state.Answer.Streamed {
		return reply
	}
	if state != nil && (state.Session.NeedHandoff || state.Session.AwaitingUser || state.Session.AwaitingConfirm) {
		return reply
	}
	notice := "\u5f53\u524d\u7ed3\u679c\u7f6e\u4fe1\u5ea6\u8f83\u4f4e\uff0c\u5efa\u8bae\u4f60\u518d\u8865\u5145\u4e00\u70b9\u4fe1\u606f\uff0c\u6211\u4f1a\u7ee7\u7eed\u5e2e\u4f60\u786e\u8ba4\u3002"
	if strings.Contains(reply, notice) {
		return reply
	}
	return strings.TrimSpace(reply + "\n\n" + notice)
}

func (n *FinalizeNode) emitReply(ctx context.Context, state *graphstate.State, reply string) error {
	if state == nil || state.StreamWriter == nil || state.Answer.Streamed || strings.TrimSpace(reply) == "" {
		return nil
	}
	orchestratorobserve.SendEvent(ctx, state.StreamWriter, "token", map[string]any{
		"trace_id": state.TraceID,
		"text":     reply,
	})
	state.Answer.Streamed = true
	return nil
}

func (n *FinalizeNode) persistTurn(ctx context.Context, state *graphstate.State, resp *domain.ChatResult) {
	if state == nil || resp == nil || state.SessionMeta == nil {
		return
	}

	userMsg := agentsession.NewUserMessage(state.SessionMeta.SessionID, state.Session.RawQuery)
	assistantMsg := agentsession.NewAssistantMessage(state.SessionMeta.SessionID, resp.Reply, resp.Intent, resp.Confidence)
	if strings.TrimSpace(userMsg.Content) == "" && strings.TrimSpace(assistantMsg.Content) == "" {
		return
	}

	sessionSnapshot := *state.SessionMeta
	sessionSnapshot.Slots = mergePersistedSessionState(state.Session.Slots, state.Session.CurrentRefs, state.Session.PendingSelections)
	sessionSnapshot.TotalTurns++
	sessionSnapshot.UpdatedAt = assistantMsg.CreatedAt
	if sessionSnapshot.UpdatedAt.IsZero() {
		sessionSnapshot.UpdatedAt = time.Now()
	}
	if strings.TrimSpace(assistantMsg.Content) != "" {
		sessionSnapshot.LastMessage = agentsession.Truncate(assistantMsg.Content, 64)
	} else {
		sessionSnapshot.LastMessage = agentsession.Truncate(userMsg.Content, 64)
	}
	if sessionSnapshot.Status == "" {
		sessionSnapshot.Status = domain.SessionStatusActive
	}
	state.SessionMeta = &sessionSnapshot

	if n.SessionService != nil {
		history := agentsession.FromSchemaMessages(sessionSnapshot.SessionID, append([]*schema.Message(nil), state.Session.Messages...))
		if strings.TrimSpace(userMsg.Content) != "" {
			history = append(history, userMsg)
		}
		if strings.TrimSpace(assistantMsg.Content) != "" {
			history = append(history, assistantMsg)
		}
		if err := n.SessionService.SaveTurnCache(ctx, sessionSnapshot, history); err != nil && n.Logger != nil {
			n.Logger.Warn("save session cache failed", logger.Error(err))
		}
	}

	if n.SessionService == nil && n.CacheWriteback == nil {
		return
	}

	go n.persistInBackground(context.WithoutCancel(ctx), sessionSnapshot, userMsg, assistantMsg, cloneConversationState(state))
}

func (n *FinalizeNode) persistInBackground(
	ctx context.Context,
	session domain.Session,
	userMsg domain.Message,
	assistantMsg domain.Message,
	state *graphstate.State,
) {
	bgCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if n.SessionService != nil {
		if err := n.SessionService.SaveTurnPersistent(bgCtx, session, userMsg, assistantMsg); err != nil && n.Logger != nil {
			n.Logger.Warn("persist session turn failed", logger.Error(err))
		}
	}
	if n.CacheWriteback != nil {
		if err := n.CacheWriteback.Write(bgCtx, state); err != nil && n.Logger != nil {
			n.Logger.Warn("write answer cache failed", logger.Error(err))
		}
	}
}

func (n *FinalizeNode) selectedSkillText(names []string) string {
	if n.Skills == nil {
		return "none"
	}
	return agentskill.RenderSkillSummaryText(n.Skills.SummariesByNames(names))
}

func (n *FinalizeNode) selectedToolText(state *graphstate.State) string {
	if n.Tools == nil {
		return "none"
	}
	return agenttool.RenderToolSummaryText(n.Tools.Summaries(support.SelectedToolNames(state)))
}

func cloneConversationState(state *graphstate.State) *graphstate.State {
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
	cloned.Retrieval = graphstate.RetrievalResult{
		Documents: append([]*schema.Document(nil), state.Retrieval.Documents...),
	}
	cloned.Tool = graphstate.ToolState{
		Plans:        append([]domain.ToolCallPlan(nil), state.Tool.Plans...),
		CallMessage:  state.Tool.CallMessage,
		ToolMessages: append([]*schema.Message(nil), state.Tool.ToolMessages...),
	}
	cloned.Answer = state.Answer
	if state.Interrupt != nil {
		cloned.Interrupt = &graphstate.InterruptState{
			Payload: cloneSessionSlots(state.Interrupt.Payload),
		}
	}
	if state.Response != nil {
		resp := *state.Response
		resp.References = append([]domain.KnowledgeRef(nil), state.Response.References...)
		resp.ToolExecutions = append([]domain.ToolExecution(nil), state.Response.ToolExecutions...)
		resp.Trace.Steps = append([]domain.TraceStep(nil), state.Response.Trace.Steps...)
		cloned.Response = &resp
	}
	if state.SessionMeta != nil {
		meta := *state.SessionMeta
		meta.Slots = cloneSessionSlots(state.SessionMeta.Slots)
		cloned.SessionMeta = &meta
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
