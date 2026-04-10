package global

import (
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

func TestResolveSemanticCachePolicy(t *testing.T) {
	tests := []struct {
		name       string
		route      domain.WorkflowRoute
		message    string
		wantAllow  bool
		wantBucket string
		wantScope  cache.CacheScope
	}{
		{
			name:       "return policy route always allows semantic cache",
			route:      domain.RouteReturnPolicy,
			message:    "how do I return this order",
			wantAllow:  true,
			wantBucket: string(domain.IntentReturnPolicy),
			wantScope:  cache.CacheScopeTenantPublic,
		},
		{
			name:       "stable product knowledge allows semantic cache",
			route:      domain.RouteProductInfo,
			message:    "what material is this product made of",
			wantAllow:  true,
			wantBucket: string(domain.IntentProductInfo),
			wantScope:  cache.CacheScopeTenantPublic,
		},
		{
			name:      "dynamic product question skips semantic cache",
			route:     domain.RouteProductInfo,
			message:   "what is the price of this product",
			wantAllow: false,
		},
		{
			name:       "base qa knowledge question can use semantic cache",
			route:      domain.RouteBaseQA,
			message:    "what are your shipping time rules",
			wantAllow:  true,
			wantBucket: string(domain.IntentFallback),
			wantScope:  cache.CacheScopeTenantPublic,
		},
		{
			name:      "base qa dynamic order status skips semantic cache",
			route:     domain.RouteBaseQA,
			message:   "check my order status",
			wantAllow: false,
		},
		{
			name:      "action route never allows semantic cache",
			route:     domain.RouteAddToCart,
			message:   "add this to my cart",
			wantAllow: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveSemanticCachePolicy(tc.route, tc.message)
			if got.AllowRead != tc.wantAllow {
				t.Fatalf("AllowRead = %v, want %v", got.AllowRead, tc.wantAllow)
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
