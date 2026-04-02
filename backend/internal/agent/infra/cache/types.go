package cache

import (
	"context"
	"time"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
)

type SemanticCacheItem struct {
	ID        string
	Query     string
	Reply     string
	Intent    dto.Intent
	Vector    []float64
	CreatedAt time.Time
}

type ExactCacheItem struct {
	TenantID  string
	UserID    int64
	Query     string
	Response  dto.ChatResponse
	CreatedAt time.Time
}

type SemanticCache interface {
	Lookup(ctx context.Context, vector []float64, threshold float64, limit int) (*SemanticCacheItem, error)
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
