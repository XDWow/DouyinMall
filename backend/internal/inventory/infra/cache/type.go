package cache

import "context"

type InventoryCache interface {
	IncrBy(ctx context.Context, key string, delta int32) (int64, error)
}
