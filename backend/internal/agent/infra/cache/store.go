package cache

import (
	"context"
	"time"
)

type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	ListRange(ctx context.Context, key string, start, stop int64) ([][]byte, error)
	ReplaceList(ctx context.Context, key string, values [][]byte, ttl time.Duration) error
	HashSet(ctx context.Context, key string, fields map[string]any, ttl time.Duration) error
	HashGetAll(ctx context.Context, key string) (map[string]string, error)
	Search(ctx context.Context, index string, args ...any) (any, error)
	CreateVectorIndex(ctx context.Context, spec VectorIndexSpec) error
	IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

type VectorIndexSpec struct {
	Name           string
	Prefix         string
	Dimension      int
	DistanceMetric string
}
