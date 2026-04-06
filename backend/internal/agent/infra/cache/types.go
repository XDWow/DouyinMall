package cache

import (
	"context"
	"time"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type CacheScope string

const (
	// CacheScopeTenantPublic 表示同一租户下可共享的公共知识缓存。
	CacheScopeTenantPublic CacheScope = "tenant_public"
	// CacheScopeTenantUser 表示只能给当前用户复用的个性化缓存。
	CacheScopeTenantUser CacheScope = "tenant_user"
)

// SemanticCacheLookup 描述语义缓存查询条件。
// 语义缓存不是查一个全局大池，而是先按 tenant/scope/intent bucket 缩小候选集合，
// 再在这个集合里做向量相似度匹配，减少跨业务场景误命中。
type SemanticCacheLookup struct {
	TenantID     string
	UserID       int64
	IntentBucket string
	Scope        CacheScope
	Vector       []float64
	Threshold    float64
	Limit        int
}

// SemanticCacheItem 表示一条可复用的语义缓存记录。
// Query 和 Response 保留原始问答，Vector 只用于相似度查找。
type SemanticCacheItem struct {
	ID           string
	TenantID     string
	UserID       int64
	IntentBucket string
	Scope        CacheScope
	Query        string
	Response     domain.ChatResult
	Vector       []float64
	CreatedAt    time.Time
}

type ExactCacheItem struct {
	TenantID  string
	UserID    int64
	Query     string
	Response  domain.ChatResult
	CreatedAt time.Time
}

type SemanticCache interface {
	Lookup(ctx context.Context, req SemanticCacheLookup) (*SemanticCacheItem, error)
	Store(ctx context.Context, item *SemanticCacheItem, ttl time.Duration) error
}

type ExactCache interface {
	Lookup(ctx context.Context, tenantID string, userID int64, query string) (*ExactCacheItem, error)
	Store(ctx context.Context, item *ExactCacheItem, ttl time.Duration) error
}

type RateLimiter interface {
	AllowUser(ctx context.Context, userID int64, limit int64, window time.Duration) (bool, error)
}

type CheckpointStore interface {
	compose.CheckPointStore
}
