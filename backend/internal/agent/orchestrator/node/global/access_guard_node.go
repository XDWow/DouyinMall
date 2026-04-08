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

type AccessGuardNode struct {
	DefaultTenantID    string
	RateLimitPerMinute int64
	RateLimiter        cache.RateLimiter
	CheckpointStore    cache.CheckpointStore
}

func NewAccessGuardNode(defaultTenantID string, rateLimitPerMinute int64, rateLimiter cache.RateLimiter, checkpointStore cache.CheckpointStore) *AccessGuardNode {
	return &AccessGuardNode{
		DefaultTenantID:    defaultTenantID,
		RateLimitPerMinute: rateLimitPerMinute,
		RateLimiter:        rateLimiter,
		CheckpointStore:    checkpointStore,
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
		return nil, fmt.Errorf("消息为空")
	}
	if input.UserID <= 0 {
		return nil, fmt.Errorf("用户不存在")
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
		if n.CheckpointStore == nil {
			return nil, fmt.Errorf("检查点恢复不可用")
		}
		_, ok, err := n.CheckpointStore.Get(ctx, input.ResumeToken)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("检查点不存在")
		}
		result.ResumeFromCP = true
	}

	return result, nil
}
