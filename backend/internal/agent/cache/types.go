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

type SemanticCache interface {
	Lookup(ctx context.Context, vector []float64, threshold float64, limit int) (*SemanticCacheItem, error)
	Store(ctx context.Context, item *SemanticCacheItem, ttl time.Duration) error
}

type RateLimiter interface {
	AllowUser(ctx context.Context, userID int64, limit int64, window time.Duration) (bool, error)
}

type CheckpointStore interface {
	compose.CheckPointStore
}

