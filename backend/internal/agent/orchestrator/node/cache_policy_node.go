package node

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// CachePolicyInput 描述缓存策略真正依赖的最小输入。
type CachePolicyInput struct {
	TenantID        string
	UserID          int64
	Message         string
	ResumeFromCP    bool
	AwaitingUser    bool
	AwaitingConfirm bool
}

// CachePolicyResult 只负责给后续多级缓存节点提供策略，不做正式意图识别。
type CachePolicyResult struct {
	AllowExact    bool
	AllowSemantic bool
	IntentBucket  string
	Scope         cache.CacheScope
}

// CachePolicyNode 先做轻量缓存分流，能确定的策略交给编排层，不提前跑完整主流程。
// 这里使用的是“缓存专用中文分类”，只用于决定能否走缓存和落到哪个缓存桶，
// 不替代后面的正式意图识别。
type CachePolicyNode struct{}

func NewCachePolicyNode() *CachePolicyNode {
	return &CachePolicyNode{}
}

func (n *CachePolicyNode) Invoke(_ context.Context, input CachePolicyInput) (*CachePolicyResult, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" || input.ResumeFromCP || input.AwaitingUser || input.AwaitingConfirm {
		return &CachePolicyResult{}, nil
	}

	intent := support.DetectCacheIntent(message)
	result := &CachePolicyResult{
		AllowExact:   true,
		IntentBucket: support.CacheIntentBucket(intent),
		Scope:        support.CacheScopeForIntent(intent),
	}

	switch intent {
	case domain.IntentAddToCart, domain.IntentReturnExchangeApply:
		result.AllowExact = false
		result.AllowSemantic = false
	default:
		result.AllowSemantic = support.AllowSemanticCache(intent, message)
	}

	return result, nil
}
