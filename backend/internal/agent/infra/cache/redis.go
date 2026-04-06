package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	semanticIndexKeyPrefix = "agent:semantic:index"
	semanticItemKeyPrefix  = "agent:semantic:item:"
	exactItemKeyPrefix     = "agent:exact:item:"
	checkpointKeyPrefix    = "agent:checkpoint:"
	rateKeyPrefix          = "agent:rate:"
)

type RedisExactCache struct {
	rdb redis.Cmdable
}

func NewRedisExactCache(rdb redis.Cmdable) *RedisExactCache {
	return &RedisExactCache{rdb: rdb}
}

func (c *RedisExactCache) Lookup(ctx context.Context, tenantID string, userID int64, query string) (*ExactCacheItem, error) {
	key := exactCacheKey(tenantID, userID, query)
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var item ExactCacheItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *RedisExactCache) Store(ctx context.Context, item *ExactCacheItem, ttl time.Duration) error {
	if item == nil {
		return nil
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, exactCacheKey(item.TenantID, item.UserID, item.Query), raw, ttl).Err()
}

type RedisSemanticCache struct {
	rdb      redis.Cmdable
	maxItems int64
}

func NewRedisSemanticCache(rdb redis.Cmdable) *RedisSemanticCache {
	return &RedisSemanticCache{
		rdb:      rdb,
		maxItems: 256,
	}
}

func (c *RedisSemanticCache) Lookup(ctx context.Context, req SemanticCacheLookup) (*SemanticCacheItem, error) {
	if len(req.Vector) == 0 {
		return nil, nil
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	// Redis 这一层的“分桶”就是先按 tenant + scope + intent bucket 拿到对应索引，
	// 避免把所有语义缓存都混在一起比较。
	indexKey := semanticIndexKey(req.TenantID, req.UserID, req.IntentBucket, req.Scope)

	ids, err := c.rdb.ZRevRange(ctx, indexKey, 0, int64(req.Limit-1)).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	type candidate struct {
		item  *SemanticCacheItem
		score float64
	}
	candidates := make([]candidate, 0, len(ids))
	for _, id := range ids {
		raw, err := c.rdb.Get(ctx, semanticItemKeyPrefix+id).Bytes()
		if err != nil {
			continue
		}

		var item SemanticCacheItem
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		if !matchesSemanticLookup(item, req) {
			continue
		}
		score := cosineSimilarity(item.Vector, req.Vector)
		if score >= req.Threshold {
			candidates = append(candidates, candidate{item: &item, score: score})
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].item, nil
}

func (c *RedisSemanticCache) Store(ctx context.Context, item *SemanticCacheItem, ttl time.Duration) error {
	if item == nil {
		return nil
	}
	if item.ID == "" {
		item.ID = hashVector(item.Vector, item.Query)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}

	pipe := c.rdb.Pipeline()
	pipe.Set(ctx, semanticItemKeyPrefix+item.ID, payload, ttl)
	pipe.ZAdd(ctx, semanticIndexKey(item.TenantID, item.UserID, item.IntentBucket, item.Scope), redis.Z{
		Score:  float64(item.CreatedAt.UnixMilli()),
		Member: item.ID,
	})
	pipe.ZRemRangeByRank(ctx, semanticIndexKey(item.TenantID, item.UserID, item.IntentBucket, item.Scope), 0, -(c.maxItems + 1))
	_, err = pipe.Exec(ctx)
	return err
}

type RedisRateLimiter struct {
	rdb redis.Cmdable
}

func NewRedisRateLimiter(rdb redis.Cmdable) *RedisRateLimiter {
	return &RedisRateLimiter{rdb: rdb}
}

func (r *RedisRateLimiter) AllowUser(ctx context.Context, userID int64, limit int64, window time.Duration) (bool, error) {
	key := fmt.Sprintf("%s%d", rateKeyPrefix, userID)
	count, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		_ = r.rdb.Expire(ctx, key, window).Err()
	}
	return count <= limit, nil
}

type RedisCheckpointStore struct {
	rdb redis.Cmdable
	ttl time.Duration
}

func NewRedisCheckpointStore(rdb redis.Cmdable, ttl time.Duration) *RedisCheckpointStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &RedisCheckpointStore{rdb: rdb, ttl: ttl}
}

func (s *RedisCheckpointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	data, err := s.rdb.Get(ctx, checkpointKeyPrefix+checkPointID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func (s *RedisCheckpointStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	return s.rdb.Set(ctx, checkpointKeyPrefix+checkPointID, checkPoint, s.ttl).Err()
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func hashVector(vector []float64, query string) string {
	raw, _ := json.Marshal(struct {
		Query  string    `json:"query"`
		Vector []float64 `json:"vector"`
	}{
		Query:  query,
		Vector: vector,
	})
	sum := sha1.Sum(raw)
	return hex.EncodeToString(sum[:8])
}

func exactCacheKey(tenantID string, userID int64, query string) string {
	normalized := normalizeExactQuery(query)
	raw := fmt.Sprintf("%s:%d:%s", tenantID, userID, normalized)
	sum := sha1.Sum([]byte(raw))
	return exactItemKeyPrefix + hex.EncodeToString(sum[:])
}

func normalizeExactQuery(query string) string {
	query = strings.NewReplacer(
		"，", " ",
		"。", " ",
		"？", " ",
		"！", " ",
		"：", " ",
		"；", " ",
		",", " ",
		".", " ",
		"?", " ",
		"!", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"（", " ",
		"）", " ",
	).Replace(query)
	query = strings.ToLower(strings.TrimSpace(query))
	return strings.Join(strings.Fields(query), " ")
}

func semanticIndexKey(tenantID string, userID int64, intentBucket string, scope CacheScope) string {
	bucket := strings.TrimSpace(intentBucket)
	if bucket == "" {
		bucket = "fallback"
	}
	raw := fmt.Sprintf("%s:%s:%s:%d", scope, tenantID, bucket, scopedUserID(scope, userID))
	sum := sha1.Sum([]byte(raw))
	return semanticIndexKeyPrefix + ":" + hex.EncodeToString(sum[:8])
}

func scopedUserID(scope CacheScope, userID int64) int64 {
	if scope == CacheScopeTenantUser {
		return userID
	}
	return 0
}

func matchesSemanticLookup(item SemanticCacheItem, req SemanticCacheLookup) bool {
	if item.TenantID != req.TenantID {
		return false
	}
	if item.Scope != req.Scope {
		return false
	}
	if strings.TrimSpace(item.IntentBucket) != strings.TrimSpace(req.IntentBucket) {
		return false
	}
	return scopedUserID(item.Scope, item.UserID) == scopedUserID(req.Scope, req.UserID)
}
