package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type AccessGuardNode struct{ suite *Suite }
type SessionLoadNode struct{ suite *Suite }
type L0ExactCacheNode struct{ suite *Suite }
type RouteNode struct{ suite *Suite }
type ResponseRenderNode struct{ suite *Suite }
type CacheWritebackNode struct{ suite *Suite }

func (s *Suite) AccessGuard() *AccessGuardNode       { return &AccessGuardNode{suite: s} }
func (s *Suite) SessionLoad() *SessionLoadNode       { return &SessionLoadNode{suite: s} }
func (s *Suite) L0ExactCache() *L0ExactCacheNode     { return &L0ExactCacheNode{suite: s} }
func (s *Suite) Route() *RouteNode                   { return &RouteNode{suite: s} }
func (s *Suite) ResponseRender() *ResponseRenderNode { return &ResponseRenderNode{suite: s} }
func (s *Suite) CacheWriteback() *CacheWritebackNode { return &CacheWritebackNode{suite: s} }

func (n *AccessGuardNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	if strings.TrimSpace(flow.Request.Message) == "" {
		return nil, fmt.Errorf("message is required")
	}
	if flow.Request.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	state := graphstate.EnsureSessionState(flow)
	state.UserID = flow.Request.UserID
	state.RawQuery = strings.TrimSpace(flow.Request.Message)
	state.TenantID = n.suite.deps.Config.DefaultTenantID
	if state.TenantID == "" {
		state.TenantID = "default"
	}
	if n.suite.deps.RateLimiter != nil {
		allowed, err := n.suite.deps.RateLimiter.AllowUser(ctx, flow.Request.UserID, n.suite.deps.Config.RateLimitPerMinute, time.Minute)
		if err == nil && !allowed {
			state.NeedHandoff = true
			state.HandoffReason = "rate_limit"
			state.FinalAnswer = "Too many requests. Please retry later or hand off to a human agent."
			state.Route = graphstate.RouteFallback
		}
	}
	if strings.TrimSpace(flow.Request.ResumeToken) != "" {
		if n.suite.deps.CheckpointStore == nil {
			return nil, fmt.Errorf("resume is not enabled")
		}
		_, ok, err := n.suite.deps.CheckpointStore.Get(ctx, flow.Request.ResumeToken)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("resume checkpoint not found")
		}
		state.ResumeFromCP = true
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *SessionLoadNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	sessionID := strings.TrimSpace(flow.Request.SessionID)
	if sessionID == "" {
		sessionID = "sess_" + flow.TraceID
		flow.Request.SessionID = sessionID
	}
	session := flow.Session
	if session == nil {
		session = &memory.Session{ID: sessionID, UserID: flow.Request.UserID, Channel: support.FirstNonEmpty(flow.Request.Channel, "grpc"), Status: dto.SessionStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	if n.suite.deps.SessionStore != nil {
		stored, err := n.suite.deps.SessionStore.Load(ctx, sessionID)
		if err == nil && stored != nil {
			if stored.UserID != 0 && stored.UserID != flow.Request.UserID {
				return nil, fmt.Errorf("session owner mismatch")
			}
			session = stored
		}
	}
	if session.Channel == "" {
		session.Channel = support.FirstNonEmpty(flow.Request.Channel, "grpc")
	}
	if session.Status == "" {
		session.Status = dto.SessionStatusActive
	}
	flow.Session = session
	state := graphstate.EnsureSessionState(flow)
	state.SessionID = session.ID
	if graphstate.SlotString(flow, "order_id") == "" {
		if id := support.MetadataValue(flow.Request.Metadata, "order_id", "orderID"); id != "" {
			graphstate.SetSlot(flow, "order_id", support.DigitsOnlyID(id))
		}
	}
	if graphstate.SlotString(flow, "product_id") == "" {
		if id := support.MetadataValue(flow.Request.Metadata, "product_id", "productID"); id != "" {
			graphstate.SetSlot(flow, "product_id", support.DigitsOnlyID(id))
		}
	}
	if summary := strings.TrimSpace(session.Summary); summary != "" {
		graphstate.SetSlot(flow, "session_summary", summary)
	}
	flow.EnsureResponse().SessionID = session.ID
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *L0ExactCacheNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil || n.suite.deps.ExactCache == nil || flow.State.ResumeFromCP {
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	item, err := n.suite.deps.ExactCache.Lookup(ctx, flow.State.TenantID, flow.Request.UserID, flow.State.RawQuery)
	if err != nil || item == nil {
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	resp := item.Response
	resp.Trace.TraceID = flow.TraceID
	resp.Trace.CheckpointID = flow.Checkpoint
	resp.Trace.CacheHit = true
	resp.SessionID = flow.Request.SessionID
	flow.Response = &resp
	flow.State.CacheHitLevel = "L0"
	flow.State.FinalAnswer = resp.Reply
	flow.State.Intent = resp.Intent
	flow.State.Route = support.RouteFromIntent(resp.Intent)
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *RouteNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	state := graphstate.EnsureSessionState(flow)
	route := support.RouteFromIntent(flow.State.Intent)
	if state.AwaitingConfirm {
		route = graphstate.RouteReturnExchangeApply
	}
	if !support.RouteEnabled(state.FeatureFlags, route) {
		route = graphstate.RouteFallback
		state.ErrorCode = "feature_disabled"
	}
	state.Route = route
	state.ReadOnly = route != graphstate.RouteReturnExchangeApply
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *ResponseRenderNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	resp := flow.EnsureResponse()
	resp.SessionID = flow.State.SessionID
	if flow.State.CacheHitLevel == "L0" && strings.TrimSpace(resp.Reply) != "" {
		resp.Status = dto.ReplyStatusAnswered
		resp.Intent = flow.State.Intent
		graphstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}
	reply := strings.TrimSpace(flow.State.FinalAnswer)
	source := "state"
	if reply == "" && support.ShouldUseLLMAnswer(flow) && n.suite.deps.Model != nil && n.suite.deps.Prompts != nil && n.suite.deps.Prompts.Answer != nil {
		messages, err := n.suite.deps.Prompts.Answer.Format(ctx, map[string]any{"system_text": n.suite.deps.Prompts.SystemText, "history": support.HistoryMessages(flow.Session, n.suite.deps.Config.ConversationWindow), "message": flow.State.RawQuery, "query": support.FirstNonEmpty(flow.State.RewrittenQuery, flow.State.RawQuery), "references_text": support.ReferencesText(flow.Retrieval.References), "tool_text": support.ToolText(flow.ToolExecutions())})
		if err == nil && n.suite.deps.Hooks.GenerateAnswer != nil {
			if generated, genErr := n.suite.deps.Hooks.GenerateAnswer(ctx, flow, messages); genErr == nil && strings.TrimSpace(generated) != "" {
				reply = generated
				source = "llm"
			}
		}
	}
	if strings.TrimSpace(reply) == "" {
		reply = support.TemplateAnswer(flow)
		source = "template"
	}
	reply = support.NormalizeReply(reply)
	flow.Answer = graphstate.AnswerResult{Reply: reply, Confidence: support.EstimateConfidence(flow), Source: source}
	resp.Reply = reply
	resp.Intent = flow.State.Intent
	resp.Confidence = flow.Answer.Confidence
	resp.References = flow.Retrieval.References
	resp.ToolExecutions = flow.ToolExecutions()
	resp.NeedHandoff = flow.State.NeedHandoff
	resp.HandoffReason = flow.State.HandoffReason
	resp.Trace.RewrittenQuery = flow.State.RewrittenQuery
	if resp.NeedHandoff {
		resp.Status = dto.ReplyStatusHandoff
		if n.suite.deps.Metrics != nil {
			n.suite.deps.Metrics.ObserveHandoff(resp.HandoffReason)
		}
	} else {
		resp.Status = dto.ReplyStatusAnswered
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *CacheWritebackNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	resp := flow.EnsureResponse()
	resp.Trace.TraceID = flow.TraceID
	resp.Trace.CheckpointID = flow.Checkpoint
	resp.Trace.CacheHit = flow.State.CacheHitLevel != ""
	resp.Trace.RewrittenQuery = flow.State.RewrittenQuery
	if n.suite.deps.Hooks.PersistConversationTurn != nil {
		if err := n.suite.deps.Hooks.PersistConversationTurn(ctx, flow, resp.Reply, resp.Intent, resp.Confidence); err != nil {
			n.suite.deps.Logger.Warn("save session failed", logger.Error(err))
		}
	}
	if n.suite.deps.ExactCache != nil && flow.State.ReadOnly && flow.State.CacheHitLevel == "" && !resp.NeedHandoff && !flow.State.AwaitingUser && !flow.State.AwaitingConfirm && strings.TrimSpace(resp.Reply) != "" {
		_ = n.suite.deps.ExactCache.Store(ctx, &cache.ExactCacheItem{TenantID: flow.State.TenantID, UserID: flow.Request.UserID, Query: flow.State.RawQuery, Response: *resp}, n.suite.deps.Config.L0CacheTTL)
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}
