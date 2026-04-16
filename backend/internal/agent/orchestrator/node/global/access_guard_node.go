package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

type AccessGuardInput struct {
	Message     string
	UserID      int64
	ResumeToken string
}

// AccessGuardNode 入口：租户、限流。Checkpoint 存在性由 Runtime 在 Invoke 前校验。
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

type AccessGuardResult struct {
	RawQuery     string
	TenantID     string
	ResumeFromCP bool
	ErrorCode    string
	FinalAnswer  string
}

func (n *AccessGuardNode) Invoke(ctx context.Context, input AccessGuardInput) (*AccessGuardResult, error) {
	if strings.TrimSpace(input.Message) == "" {
		return nil, fmt.Errorf("empty message")
	}
	if input.UserID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}

	result := &AccessGuardResult{
		RawQuery: strings.TrimSpace(input.Message),
		TenantID: strings.TrimSpace(n.DefaultTenantID),
	}
	if result.TenantID == "" {
		result.TenantID = "default"
	}

	if n.RateLimiter != nil {
		allowed, err := n.RateLimiter.AllowUser(ctx, input.UserID, n.RateLimitPerMinute, time.Minute)
		if err == nil && !allowed {
			result.ErrorCode = "rate_limit"
			result.FinalAnswer = "发送消息太频繁，请稍后再试"
		}
	}

	if strings.TrimSpace(input.ResumeToken) != "" {
		result.ResumeFromCP = true
	}

	return result, nil
}
