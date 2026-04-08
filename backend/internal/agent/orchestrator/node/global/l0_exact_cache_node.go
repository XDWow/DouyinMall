package global

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

type L0ExactCacheInput struct {
	TenantID     string
	UserID       int64
	RawQuery     string
	SessionID    string
	TraceID      string
	CheckpointID string
	AllowRead    bool
}

type L0ExactCacheResult struct {
	CacheHit    bool
	HitLevel    string
	Response    *domain.ChatResult
	FinalAnswer string
	Intent      domain.Intent
	Route       graphstate.WorkflowRoute
}

type L0ExactCacheNode struct {
	ExactCache cache.ExactCache
}

func NewL0ExactCacheNode(exactCache cache.ExactCache) *L0ExactCacheNode {
	return &L0ExactCacheNode{ExactCache: exactCache}
}

func (n *L0ExactCacheNode) Invoke(ctx context.Context, input L0ExactCacheInput) (*L0ExactCacheResult, error) {
	if !input.AllowRead || n.ExactCache == nil {
		return &L0ExactCacheResult{}, nil
	}

	item, err := n.ExactCache.Lookup(ctx, input.TenantID, input.UserID, input.RawQuery)
	if err != nil || item == nil {
		return &L0ExactCacheResult{}, nil
	}

	return newExactCacheResult(input, item.Response), nil
}

func newExactCacheResult(input L0ExactCacheInput, resp domain.ChatResult) *L0ExactCacheResult {
	resp.Trace.TraceID = input.TraceID
	resp.Trace.CheckpointID = input.CheckpointID
	resp.Trace.CacheHit = true
	resp.SessionID = input.SessionID

	return &L0ExactCacheResult{
		CacheHit:    true,
		HitLevel:    cacheHitExact,
		Response:    &resp,
		FinalAnswer: resp.Reply,
		Intent:      resp.Intent,
		Route:       routeFromIntent(resp.Intent),
	}
}
