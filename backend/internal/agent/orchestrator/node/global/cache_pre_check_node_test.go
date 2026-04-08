package global

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func TestCachePreCheckNodeInvoke(t *testing.T) {
	node := NewCachePreCheckNode()

	tests := []struct {
		name         string
		message      string
		wantExact    bool
		wantSemantic bool
		wantBucket   string
		wantScope    cache.CacheScope
	}{
		{
			name:         "dynamic order query bypasses cache",
			message:      "check order status",
			wantExact:    false,
			wantSemantic: false,
		},
		{
			name:         "action apply query bypasses cache",
			message:      "submit a refund request",
			wantExact:    false,
			wantSemantic: false,
		},
		{
			name:         "stable return policy query can use both cache levels",
			message:      "what is the return policy",
			wantExact:    true,
			wantSemantic: true,
			wantBucket:   string(domain.IntentReturnPolicy),
			wantScope:    cache.CacheScopeTenantPublic,
		},
		{
			name:         "stable product knowledge query can use both cache levels",
			message:      "what material is this product made of",
			wantExact:    true,
			wantSemantic: true,
			wantBucket:   string(domain.IntentProductInfo),
			wantScope:    cache.CacheScopeTenantPublic,
		},
		{
			name:         "price query is not treated as stable knowledge",
			message:      "what is the price of this product",
			wantExact:    false,
			wantSemantic: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := node.Invoke(context.Background(), CachePreCheckInput{Message: tc.message})
			if err != nil {
				t.Fatalf("Invoke() error = %v", err)
			}
			if got.AllowExact != tc.wantExact {
				t.Fatalf("AllowExact = %v, want %v", got.AllowExact, tc.wantExact)
			}
			if got.AllowSemantic != tc.wantSemantic {
				t.Fatalf("AllowSemantic = %v, want %v", got.AllowSemantic, tc.wantSemantic)
			}
			if got.IntentBucket != tc.wantBucket {
				t.Fatalf("IntentBucket = %q, want %q", got.IntentBucket, tc.wantBucket)
			}
			if got.Scope != tc.wantScope {
				t.Fatalf("Scope = %q, want %q", got.Scope, tc.wantScope)
			}
		})
	}
}

func TestCacheWritebackServiceRespectsCacheGate(t *testing.T) {
	t.Run("dynamic query does not write exact cache", func(t *testing.T) {
		exact := &stubExactCache{}
		writer := NewCacheWritebackService(exact, nil, nil, time.Minute, time.Minute, nil)
		state := graphstate.NewState(domain.ChatCommand{
			SessionID: "sess_dynamic",
			UserID:    1,
			Message:   "check order status",
		}, nil, graphstate.InitOptions{})
		state.Session.TenantID = "tenant_1"
		state.Session.Intent = domain.IntentOrderQuery
		state.Session.ReadOnly = true
		state.Response = &domain.ChatResult{
			SessionID:  state.Request.SessionID,
			Reply:      "your order is in transit",
			Intent:     domain.IntentOrderQuery,
			Status:     domain.ReplyStatusAnswered,
			Confidence: 0.9,
		}

		if err := writer.Write(context.Background(), state); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if exact.storeCalls != 0 {
			t.Fatalf("expected no cache writes, got %d", exact.storeCalls)
		}
	})

	t.Run("stable knowledge query writes exact cache", func(t *testing.T) {
		exact := &stubExactCache{}
		writer := NewCacheWritebackService(exact, nil, nil, time.Minute, time.Minute, nil)
		state := graphstate.NewState(domain.ChatCommand{
			SessionID: "sess_policy",
			UserID:    2,
			Message:   "what is the return policy",
		}, nil, graphstate.InitOptions{})
		state.Session.TenantID = "tenant_1"
		state.Session.Intent = domain.IntentReturnPolicy
		state.Session.ReadOnly = true
		state.Answer.CacheableHint = testBoolPtr(true)
		state.Response = &domain.ChatResult{
			SessionID:  state.Request.SessionID,
			Reply:      "you can start a return from the order detail page",
			Intent:     domain.IntentReturnPolicy,
			Status:     domain.ReplyStatusAnswered,
			Confidence: 0.95,
		}

		if err := writer.Write(context.Background(), state); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if exact.storeCalls != 1 {
			t.Fatalf("expected one cache write, got %d", exact.storeCalls)
		}
		if exact.lastItem == nil {
			t.Fatal("expected stored cache item")
		}
		if exact.lastItem.Query != "what is the return policy" {
			t.Fatalf("stored query = %q", exact.lastItem.Query)
		}
	})

	t.Run("product query touching dynamic tool does not write cache", func(t *testing.T) {
		exact := &stubExactCache{}
		writer := NewCacheWritebackService(exact, nil, nil, time.Minute, time.Minute, nil)
		state := graphstate.NewState(domain.ChatCommand{
			SessionID: "sess_product_dynamic",
			UserID:    3,
			Message:   "is this product in stock",
		}, nil, graphstate.InitOptions{})
		state.Session.TenantID = "tenant_1"
		state.Session.Intent = domain.IntentProductInfo
		state.Session.ReadOnly = true
		state.Tool.Plans = []domain.ToolCallPlan{{Name: "get_inventory"}}
		state.Response = &domain.ChatResult{
			SessionID:  state.Request.SessionID,
			Reply:      "current stock is 12",
			Intent:     domain.IntentProductInfo,
			Status:     domain.ReplyStatusAnswered,
			Confidence: 0.93,
		}

		if err := writer.Write(context.Background(), state); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if exact.storeCalls != 0 {
			t.Fatalf("expected no cache writes, got %d", exact.storeCalls)
		}
	})

	t.Run("explicit non-cacheable hint skips stable reply", func(t *testing.T) {
		exact := &stubExactCache{}
		writer := NewCacheWritebackService(exact, nil, nil, time.Minute, time.Minute, nil)
		state := graphstate.NewState(domain.ChatCommand{
			SessionID: "sess_no_cache",
			UserID:    4,
			Message:   "what is the return policy",
		}, nil, graphstate.InitOptions{})
		state.Session.TenantID = "tenant_1"
		state.Session.Intent = domain.IntentReturnPolicy
		state.Session.ReadOnly = true
		state.Answer.CacheableHint = testBoolPtr(false)
		state.Response = &domain.ChatResult{
			SessionID:  state.Request.SessionID,
			Reply:      "you can start a return from the order detail page",
			Intent:     domain.IntentReturnPolicy,
			Status:     domain.ReplyStatusAnswered,
			Confidence: 0.95,
		}

		if err := writer.Write(context.Background(), state); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if exact.storeCalls != 0 {
			t.Fatalf("expected no cache writes, got %d", exact.storeCalls)
		}
	})
}

type stubExactCache struct {
	storeCalls int
	lastItem   *cache.ExactCacheItem
}

func (s *stubExactCache) Lookup(context.Context, string, int64, string) (*cache.ExactCacheItem, error) {
	return nil, nil
}

func (s *stubExactCache) Store(_ context.Context, item *cache.ExactCacheItem, _ time.Duration) error {
	s.storeCalls++
	s.lastItem = item
	return nil
}

func testBoolPtr(value bool) *bool {
	return &value
}
