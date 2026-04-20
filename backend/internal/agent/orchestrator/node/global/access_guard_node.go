package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

type AccessGuardInput struct {
	UserID      int64
	Message     string
	ResumeToken string
}

type AccessGuardResult struct {
	Blocked  bool
	Response *domain.ChatResult
}

type AccessGuardNode struct {
	DefaultTenantID    string
	RateLimitPerMinute int64
	RateLimiter        cache.RateLimiter
}

func NewAccessGuardNode(defaultTenantID string, rateLimitPerMinute int64, rateLimiter cache.RateLimiter) *AccessGuardNode {
	return &AccessGuardNode{
		DefaultTenantID:    defaultTenantID,
		RateLimitPerMinute: rateLimitPerMinute,
		RateLimiter:        rateLimiter,
	}
}

func (n *AccessGuardNode) Invoke(ctx context.Context, in AccessGuardInput) (AccessGuardResult, error) {
	if in.UserID <= 0 {
		return AccessGuardResult{}, fmt.Errorf("user_id is required")
	}

	if strings.TrimSpace(in.Message) == "" && strings.TrimSpace(in.ResumeToken) == "" {
		return AccessGuardResult{
			Blocked: true,
			Response: &domain.ChatResult{
				Status: domain.ReplyStatusBlocked,
				Reply:  "message is required",
			},
		}, nil
	}

	if n != nil && n.RateLimiter != nil && in.UserID > 0 {
		allowed, err := n.RateLimiter.AllowUser(ctx, in.UserID, n.RateLimitPerMinute, time.Minute)
		if err != nil {
			return AccessGuardResult{}, err
		}
		if !allowed {
			return AccessGuardResult{
				Blocked: true,
				Response: &domain.ChatResult{
					Status: domain.ReplyStatusBlocked,
					Reply:  "发送消息过于频繁，请稍后再试",
				},
			}, nil
		}
	}

	return AccessGuardResult{}, nil
}
