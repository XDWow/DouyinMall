package global

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

type CachePreCheckInput struct {
	TenantID        string
	UserID          int64
	Message         string
	ResumeFromCP    bool
	AwaitingUser    bool
	AwaitingConfirm bool
}

type CachePreCheckResult struct {
	AllowExact    bool
	AllowSemantic bool
	IntentBucket  string
	Scope         cache.CacheScope
}

// CachePreCheckNode 是否允许查 L0（精确缓存）。
type CachePreCheckNode struct{}

func NewCachePreCheckNode() *CachePreCheckNode {
	return &CachePreCheckNode{}
}

func (n *CachePreCheckNode) Invoke(_ context.Context, input CachePreCheckInput) (*CachePreCheckResult, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" || input.ResumeFromCP || input.AwaitingUser || input.AwaitingConfirm {
		return &CachePreCheckResult{}, nil
	}

	intent := detectCacheIntent(message)
	if !allowCacheLookup(intent, message) {
		return &CachePreCheckResult{}, nil
	}

	return &CachePreCheckResult{
		AllowExact: allowExactCache(intent, message),
	}, nil
}
