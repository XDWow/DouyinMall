package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

type AccessGuardNodeDeps struct {
	DefaultTenantID    string
	RateLimitPerMinute int64
	RateLimiter        cache.RateLimiter
	CheckpointStore    cache.CheckpointStore
}

type AccessGuardNode struct{ deps AccessGuardNodeDeps }

func NewAccessGuardNode(deps AccessGuardNodeDeps) *AccessGuardNode {
	return &AccessGuardNode{deps: deps}
}

func (n *AccessGuardNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	if strings.TrimSpace(state.Request.Message) == "" {
		return nil, fmt.Errorf("message is required")
	}
	if state.Request.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	ss := graphstate.EnsureSessionState(state)
	ss.UserID = state.Request.UserID
	ss.RawQuery = strings.TrimSpace(state.Request.Message)
	ss.TenantID = n.deps.DefaultTenantID
	if ss.TenantID == "" {
		ss.TenantID = "default"
	}
	if n.deps.RateLimiter != nil {
		allowed, err := n.deps.RateLimiter.AllowUser(ctx, state.Request.UserID, n.deps.RateLimitPerMinute, time.Minute)
		if err == nil && !allowed {
			ss.NeedHandoff = true
			ss.HandoffReason = "rate_limit"
			ss.FinalAnswer = "Too many requests. Please retry later or hand off to a human agent."
			ss.Route = graphstate.RouteFallback
		}
	}
	if strings.TrimSpace(state.Request.ResumeToken) != "" {
		if n.deps.CheckpointStore == nil {
			return nil, fmt.Errorf("resume is not enabled")
		}
		_, ok, err := n.deps.CheckpointStore.Get(ctx, state.Request.ResumeToken)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("resume checkpoint not found")
		}
		ss.ResumeFromCP = true
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
