package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

// AccessGuardInput 描述入口校验阶段的输入。
type AccessGuardInput struct {
	Message     string
	UserID      int64
	ResumeToken string
}

// AccessGuardNode 负责入口参数校验、限流和断点恢复检查。
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
	UserID        int64
	RawQuery      string
	TenantID      string
	ResumeFromCP  bool
	NeedHandoff   bool
	HandoffReason string
	FinalAnswer   string
	Route         graphstate.WorkflowRoute
}

// Invoke 完成入口校验，并返回需要写回状态的结果。
func (n *AccessGuardNode) Invoke(ctx context.Context, input AccessGuardInput) (*AccessGuardResult, error) {
	if strings.TrimSpace(input.Message) == "" {
		return nil, fmt.Errorf("消息不能为空")
	}
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user_id 不能为空")
	}

	result := &AccessGuardResult{
		UserID:   input.UserID,
		RawQuery: strings.TrimSpace(input.Message),
		TenantID: n.DefaultTenantID,
		Route:    graphstate.RouteUnknown,
	}
	if result.TenantID == "" {
		result.TenantID = "default"
	}

	if n.RateLimiter != nil {
		allowed, err := n.RateLimiter.AllowUser(ctx, input.UserID, n.RateLimitPerMinute, time.Minute)
		if err == nil && !allowed {
			result.NeedHandoff = true
			result.HandoffReason = "rate_limit"
			result.FinalAnswer = "请求过于频繁，请稍后再试或转人工处理。"
			result.Route = graphstate.RouteFallback
		}
	}

	if strings.TrimSpace(input.ResumeToken) != "" {
		if n.CheckpointStore == nil {
			return nil, fmt.Errorf("当前未开启断点恢复")
		}
		_, ok, err := n.CheckpointStore.Get(ctx, input.ResumeToken)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("未找到对应的恢复断点")
		}
		result.ResumeFromCP = true
	}

	return result, nil
}
