package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return data, err
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}

func (s *RedisStore) ListRange(ctx context.Context, key string, start, stop int64) ([][]byte, error) {
	values, err := s.client.LRange(ctx, key, start, stop).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([][]byte, 0, len(values))
	for _, value := range values {
		result = append(result, []byte(value))
	}
	return result, nil
}

func (s *RedisStore) ReplaceList(ctx context.Context, key string, values [][]byte, ttl time.Duration) error {
	pipe := s.client.Pipeline()
	pipe.Del(ctx, key)
	if len(values) > 0 {
		items := make([]any, 0, len(values))
		for _, value := range values {
			items = append(items, value)
		}
		pipe.RPush(ctx, key, items...)
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) HashSet(ctx context.Context, key string, fields map[string]any, ttl time.Duration) error {
	if len(fields) == 0 {
		return nil
	}
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, key, fields)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) HashGetAll(ctx context.Context, key string) (map[string]string, error) {
	result, err := s.client.HGetAll(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func (s *RedisStore) Search(ctx context.Context, index string, args ...any) (any, error) {
	command := make([]any, 0, len(args)+3)
	command = append(command, "FT.SEARCH", index)
	command = append(command, args...)
	return s.client.Do(ctx, command...).Result()
}

func (s *RedisStore) CreateVectorIndex(ctx context.Context, spec VectorIndexSpec) error {
	if spec.Dimension <= 0 {
		return fmt.Errorf("vector dimension must be positive")
	}
	if strings.TrimSpace(spec.DistanceMetric) == "" {
		spec.DistanceMetric = "COSINE"
	}

	err := s.client.Do(ctx,
		"FT.CREATE", spec.Name,
		"ON", "HASH",
		"PREFIX", 1, spec.Prefix,
		"SCHEMA",
		"tenant", "TAG",
		"user_id", "NUMERIC", "SORTABLE",
		"intent_bucket", "TAG",
		"scope", "TAG",
		"created_at", "NUMERIC", "SORTABLE",
		"query", "TEXT", "NOSTEM",
		"embedding", "VECTOR", "HNSW", 10,
		"TYPE", "FLOAT64",
		"DIM", spec.Dimension,
		"DISTANCE_METRIC", spec.DistanceMetric,
		"M", 16,
		"EF_CONSTRUCTION", 200,
	).Err()
	if err == nil || isIndexExistsError(err) {
		return nil
	}
	return err
}

func (s *RedisStore) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := s.client.TxPipeline()
	count := pipe.Incr(ctx, key)
	if ttl > 0 {
		pipe.ExpireNX(ctx, key, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return count.Val(), nil
}

func isIndexExistsError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "index already exists") || strings.Contains(text, "index exists")
}
