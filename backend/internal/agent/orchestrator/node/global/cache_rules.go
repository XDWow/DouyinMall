package global

import (
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
)

// TenantIDOf returns tenant id from state metadata or fallback.
func TenantIDOf(st *domain.State, fallback string) string {
	if st != nil && st.Input != nil {
		if tid := strings.TrimSpace(st.Input.Metadata["tenant_id"]); tid != "" {
			return tid
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "default"
}

// ShouldSkipCache reports whether exact/semantic cache must be skipped for this turn.
func ShouldSkipCache(st *domain.State) bool {
	if st == nil || st.Input == nil {
		return true
	}
	if strings.TrimSpace(st.Input.Message) == "" {
		return true
	}
	if strings.TrimSpace(st.Input.InterruptID) != "" {
		return true
	}
	return false
}

func CanReadCache(st *domain.State) bool {
	if ShouldSkipCache(st) {
		return false
	}
	return domain.DefaultReadOnlyForIntent(st.Intent) && (exactCacheableIntent(st.Intent) || semanticCacheableIntent(st.Intent))
}

func CanWriteCache(st *domain.State) bool {
	if !CanReadCache(st) || st.Response == nil {
		return false
	}
	if st.Response.Trace.CacheHit || st.Response.Interrupted || st.Response.Interrupt != nil {
		return false
	}
	if st.Response.NeedHandoff {
		return false
	}
	return st.Response.Status == "" || st.Response.Status == domain.ReplyStatusAnswered
}

func exactCacheableIntent(intent domain.Intent) bool {
	switch intent {
	case domain.IntentProductService, domain.IntentOrderService, domain.IntentPromotionService, domain.IntentAftersalesPolicy:
		return true
	default:
		return false
	}
}

func semanticCacheableIntent(intent domain.Intent) bool {
	switch intent {
	case domain.IntentProductService, domain.IntentOrderService, domain.IntentPromotionService, domain.IntentAftersalesPolicy:
		return true
	default:
		return false
	}
}

func semanticIntentBucket(intent domain.Intent) string {
	if !semanticCacheableIntent(intent) {
		return ""
	}
	return string(intent)
}

func semanticScopeForIntent(intent domain.Intent) cache.CacheScope {
	switch intent {
	case domain.IntentAftersalesPolicy:
		return cache.CacheScopeTenantPublic
	case domain.IntentProductService, domain.IntentOrderService, domain.IntentPromotionService:
		return cache.CacheScopeTenantUser
	default:
		return cache.CacheScopeTenantUser
	}
}
