package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

const recentTurnsWindow = 10

type FinalizeInput struct {
	AccessGuard      *domain.ChatResult
	ProductService   *domain.ChatResult
	OrderService     *domain.ChatResult
	PromotionService *domain.ChatResult
	AftersalesPolicy *domain.ChatResult
	AftersalesApply  *domain.ChatResult
	AddToCart        *domain.ChatResult
	Unknown          *domain.ChatResult
}

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

func (n *FinalizeNode) Invoke(ctx context.Context, in FinalizeInput) (*domain.State, error) {
	var out *domain.State
	if err := domain.ProcessState(ctx, func(st *domain.State) error {
		if st == nil {
			return fmt.Errorf("state is required")
		}
		out = st

		finalResp := st.EnsureResponse()
		if resp := selectFinalizeResponse(in); resp != nil {
			*finalResp = *resp
		}

		finalResp.SessionID = support.FirstNonEmpty(finalResp.SessionID, st.Session.SessionID)
		finalResp.Trace.TraceID = st.TraceID
		finalResp.Trace.RewrittenQuery = domain.EffectiveRewrittenQuery(st)
		finalResp.ToolExecutions = append([]domain.ToolExecution(nil), agenttool.ToolExecutionsFromContext(ctx)...)
		finalResp.UsedToolNames = collectToolNames(finalResp.ToolExecutions)
		if finalResp.Intent == "" {
			finalResp.Intent = st.Intent
		}
		if finalResp.Status == "" {
			if finalResp.NeedHandoff {
				finalResp.Status = domain.ReplyStatusHandoff
			} else {
				finalResp.Status = domain.ReplyStatusAnswered
			}
		}
		finalResp.Reply = support.NormalizeReply(support.FirstNonEmpty(finalResp.Reply, support.TemplateAnswer(st)))
		if finalResp.Confidence <= 0 {
			finalResp.Confidence = support.EstimateConfidence(ctx, st)
		}

		ensureRecentMessages(st, finalResp.Reply)
		return nil
	}); err != nil {
		return nil, err
	}

	if out == nil {
		return nil, fmt.Errorf("state is required")
	}

	n.emitReply(ctx, out)
	n.persistTurn(ctx, out)
	return out, nil
}

func selectFinalizeResponse(in FinalizeInput) *domain.ChatResult {
	for _, resp := range []*domain.ChatResult{
		in.AccessGuard,
		in.ProductService,
		in.OrderService,
		in.PromotionService,
		in.AftersalesPolicy,
		in.AftersalesApply,
		in.AddToCart,
		in.Unknown,
	} {
		if resp != nil {
			return resp
		}
	}
	return nil
}

func (n *FinalizeNode) emitReply(ctx context.Context, st *domain.State) {
	if st == nil || st.Response == nil || strings.TrimSpace(st.Response.Reply) == "" {
		return
	}
	writer := agenttool.StreamWriterFrom(ctx)
	if writer == nil {
		return
	}
	orchestratorobserve.SendEvent(ctx, writer, "token", map[string]any{
		"trace_id": st.TraceID,
		"text":     st.Response.Reply,
	})
	st.Response.Streamed = true
}

func (n *FinalizeNode) persistTurn(ctx context.Context, st *domain.State) {
	if n == nil || n.SessionService == nil || st == nil || st.Session == nil || st.Response == nil {
		return
	}
	sid := strings.TrimSpace(st.Session.SessionID)
	if sid == "" {
		return
	}

	snapshot, err := n.SessionService.LoadSnapshot(ctx, sid)
	if err != nil && n.Logger != nil {
		n.Logger.Warn("load session snapshot failed", logger.Error(err))
	}

	meta := domain.SessionTableMeta{
		Status:     domain.SessionStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		TotalTurns: 0,
	}
	if snapshot != nil {
		meta = snapshot.Loaded.Meta
	}
	meta.TotalTurns++
	meta.UpdatedAt = time.Now()
	meta.LastMessage = agentsession.Truncate(support.FirstNonEmpty(st.Response.Reply, st.Input.Message), 64)

	userMsg := agentsession.NewUserMessage(sid, strings.TrimSpace(st.Input.Message))
	assistantMsg := agentsession.NewAssistantMessage(sid, strings.TrimSpace(st.Response.Reply), st.Response.Intent, st.Response.Confidence)
	input := domain.RoundPersistInput{
		User:  *st.Session,
		Meta:  meta,
		Slots: domain.CloneAnyMap(st.Session.Slots),
	}

	if snapshot != nil {
		messages := append([]domain.SessionMessage(nil), snapshot.Messages...)
		if strings.TrimSpace(userMsg.Content) != "" {
			messages = append(messages, userMsg)
		}
		if strings.TrimSpace(assistantMsg.Content) != "" {
			messages = append(messages, assistantMsg)
		}
		if err := n.SessionService.SaveTurnCache(ctx, input, messages); err != nil && n.Logger != nil {
			n.Logger.Warn("save session cache failed", logger.Error(err))
		}
	}

	go func(cloned *domain.State) {
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()

		if err := n.SessionService.SaveTurnPersistent(bgCtx, input, userMsg, assistantMsg); err != nil && n.Logger != nil {
			n.Logger.Warn("save session persistent failed", logger.Error(err))
		}
		if n.CacheWriteback != nil {
			if err := n.CacheWriteback.Write(bgCtx, cloned); err != nil && n.Logger != nil {
				n.Logger.Warn("cache writeback failed", logger.Error(err))
			}
		}
	}(cloneState(st))
}

func cloneState(st *domain.State) *domain.State {
	if st == nil {
		return nil
	}
	out := *st
	if st.Session != nil {
		sess := *st.Session
		sess.RecentMessages = append([]domain.MessageTurn(nil), st.Session.RecentMessages...)
		sess.ProductList = append([]string(nil), st.Session.ProductList...)
		sess.OrderList = append([]string(nil), st.Session.OrderList...)
		sess.PromotionList = append([]string(nil), st.Session.PromotionList...)
		sess.Slots = domain.CloneAnyMap(st.Session.Slots)
		out.Session = &sess
	}
	if st.Response != nil {
		resp := *st.Response
		resp.References = append([]domain.KnowledgeRef(nil), st.Response.References...)
		resp.ToolExecutions = append([]domain.ToolExecution(nil), st.Response.ToolExecutions...)
		resp.UsedToolNames = append([]string(nil), st.Response.UsedToolNames...)
		resp.Trace.Steps = append([]domain.TraceStep(nil), st.Response.Trace.Steps...)
		out.Response = &resp
	}
	return &out
}

func ensureRecentMessages(st *domain.State, reply string) {
	if st == nil || st.Session == nil || st.Input == nil {
		return
	}
	turns := append([]domain.MessageTurn(nil), st.Session.RecentMessages...)
	if msg := strings.TrimSpace(st.Input.Message); msg != "" {
		turns = append(turns, domain.MessageTurn{Role: domain.RoleUser, Content: msg})
	}
	if reply = strings.TrimSpace(reply); reply != "" {
		turns = append(turns, domain.MessageTurn{Role: domain.RoleAssistant, Content: reply})
	}
	maxMessages := recentTurnsWindow * 2
	if maxMessages > 0 && len(turns) > maxMessages {
		turns = append([]domain.MessageTurn(nil), turns[len(turns)-maxMessages:]...)
	}
	st.Session.RecentMessages = turns
}

func collectToolNames(execs []domain.ToolExecution) []string {
	if len(execs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(execs))
	out := make([]string, 0, len(execs))
	for _, exec := range execs {
		name := strings.TrimSpace(exec.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
