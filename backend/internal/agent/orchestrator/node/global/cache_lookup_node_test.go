package global

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

type fakeExactCache struct {
	item   *cache.ExactCacheItem
	called int
}

func (f *fakeExactCache) Lookup(context.Context, string, int64, string) (*cache.ExactCacheItem, error) {
	f.called++
	return f.item, nil
}

func (f *fakeExactCache) Store(context.Context, *cache.ExactCacheItem, time.Duration) error {
	return nil
}

func TestCacheLookupBypassesWriteIntent(t *testing.T) {
	exact := &fakeExactCache{item: &cache.ExactCacheItem{Response: domain.ChatResult{
		Intent: domain.IntentAddToCart,
		Reply:  "cached write response must not be used",
	}}}
	node := NewCacheLookupNode(exact, nil, nil, 0.9, 10, "default")
	st := domain.NewState(&domain.ChatInput{UserID: 7, SessionID: "sess_1", Message: "帮我加入购物车"})
	st.Intent = domain.IntentAddToCart
	ctx := domain.WithInitialState(context.Background(), st)

	out, err := node.Invoke(ctx, CacheLookupInput{
		TenantID:  "default",
		UserID:    7,
		SessionID: "sess_1",
		TraceID:   "trace_1",
		Intent:    domain.IntentAddToCart,
		Query:     "帮我加入购物车",
	})

	require.NoError(t, err)
	require.False(t, out.Hit)
	require.Equal(t, domain.RouteAddToCart, out.Route)
	require.Equal(t, 0, exact.called)
}

func TestCacheLookupAllowsContextDependentRead(t *testing.T) {
	exact := &fakeExactCache{item: &cache.ExactCacheItem{Response: domain.ChatResult{
		Intent: domain.IntentOrderService,
		Reply:  "cached order response",
	}}}
	node := NewCacheLookupNode(exact, nil, nil, 0.9, 10, "default")
	st := domain.NewState(&domain.ChatInput{UserID: 7, SessionID: "sess_1", Message: "查第一个订单"})
	st.Intent = domain.IntentOrderService
	ctx := domain.WithInitialState(context.Background(), st)

	out, err := node.Invoke(ctx, CacheLookupInput{
		TenantID:  "default",
		UserID:    7,
		SessionID: "sess_1",
		TraceID:   "trace_1",
		Intent:    domain.IntentOrderService,
		Query:     "查第一个订单",
	})

	require.NoError(t, err)
	require.True(t, out.Hit)
	require.Equal(t, domain.RouteOrderService, out.Route)
	require.Equal(t, 1, exact.called)
	require.NotNil(t, out.Response)
	require.Equal(t, "cached order response", out.Response.Reply)
}

func TestCacheLookupReturnsExactReadHit(t *testing.T) {
	exact := &fakeExactCache{item: &cache.ExactCacheItem{Response: domain.ChatResult{
		Intent: domain.IntentProductService,
		Status: domain.ReplyStatusAnswered,
		Reply:  "cached product answer",
	}}}
	node := NewCacheLookupNode(exact, nil, nil, 0.9, 10, "default")
	st := domain.NewState(&domain.ChatInput{UserID: 7, SessionID: "sess_1", Message: "商品 123 有货吗"})
	st.Intent = domain.IntentProductService
	ctx := domain.WithInitialState(context.Background(), st)

	out, err := node.Invoke(ctx, CacheLookupInput{
		TenantID:  "default",
		UserID:    7,
		SessionID: "sess_1",
		TraceID:   "trace_1",
		Intent:    domain.IntentProductService,
		Query:     "商品 123 有货吗",
	})

	require.NoError(t, err)
	require.True(t, out.Hit)
	require.Equal(t, cacheHitExact, out.Source)
	require.Equal(t, 1, exact.called)
	require.NotNil(t, out.Response)
	require.Equal(t, "cached product answer", out.Response.Reply)
	require.Equal(t, "sess_1", out.Response.SessionID)
	require.Equal(t, "trace_1", out.Response.TraceID)
	require.True(t, out.Response.Trace.CacheHit)
}
