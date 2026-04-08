package cache

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

const (
	semanticItemKeyPrefix = "agent:semantic:item:"
	semanticIndexName     = "agent:semantic:index"
	exactItemKeyPrefix    = "agent:exact:item:"
	checkpointKeyPrefix   = "agent:checkpoint:"
	rateKeyPrefix         = "agent:rate:"
)

type RedisExactCache struct {
	store Store
}

func NewRedisExactCache(store Store) *RedisExactCache {
	return &RedisExactCache{store: store}
}

func (c *RedisExactCache) Lookup(ctx context.Context, tenantID string, userID int64, query string) (*ExactCacheItem, error) {
	if c == nil || c.store == nil {
		return nil, nil
	}
	raw, err := c.store.Get(ctx, exactCacheKey(tenantID, userID, query))
	if err != nil || len(raw) == 0 {
		return nil, err
	}

	var item ExactCacheItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *RedisExactCache) Store(ctx context.Context, item *ExactCacheItem, ttl time.Duration) error {
	if c == nil || c.store == nil || item == nil {
		return nil
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return c.store.Set(ctx, exactCacheKey(item.TenantID, item.UserID, item.Query), raw, ttl)
}

type RedisSemanticCache struct {
	store Store
}

func NewRedisSemanticCache(store Store) *RedisSemanticCache {
	return &RedisSemanticCache{store: store}
}

func (c *RedisSemanticCache) Lookup(ctx context.Context, req SemanticCacheLookup) (*SemanticCacheItem, error) {
	if c == nil || c.store == nil || len(req.Vector) == 0 {
		return nil, nil
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if err := c.ensureIndex(ctx, len(req.Vector)); err != nil {
		return nil, err
	}

	queryText := buildSemanticQuery(req)
	rawResult, err := c.store.Search(
		ctx,
		semanticIndexName,
		queryText,
		"PARAMS", 2, "vector", vectorBytes(req.Vector),
		"SORTBY", "vector_distance", "ASC",
		"RETURN", 8,
		"query", "response", "tenant", "user_id", "intent_bucket", "scope", "created_at", "vector_distance",
		"LIMIT", 0, req.Limit,
		"DIALECT", 2,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unknown index name") {
			return nil, nil
		}
		return nil, err
	}

	hits, err := parseSearchResults(rawResult)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, nil
	}

	for _, hit := range hits {
		if hit == nil || hit.Item == nil || !matchesSemanticLookup(*hit.Item, req) {
			continue
		}
		if hit.Similarity >= req.Threshold {
			hit.Item.Response.Trace.CacheHit = true
			return hit.Item, nil
		}
	}
	return nil, nil
}

func (c *RedisSemanticCache) Store(ctx context.Context, item *SemanticCacheItem, ttl time.Duration) error {
	if c == nil || c.store == nil || item == nil || len(item.Vector) == 0 {
		return nil
	}
	if item.ID == "" {
		item.ID = hashVector(item.Vector, item.Query)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if err := c.ensureIndex(ctx, len(item.Vector)); err != nil {
		return err
	}

	response, err := json.Marshal(item.Response)
	if err != nil {
		return err
	}

	return c.store.HashSet(ctx, semanticItemKeyPrefix+item.ID, map[string]any{
		"tenant":        item.TenantID,
		"user_id":       item.UserID,
		"intent_bucket": item.IntentBucket,
		"scope":         item.Scope,
		"query":         item.Query,
		"response":      response,
		"created_at":    item.CreatedAt.UnixMilli(),
		"embedding":     vectorBytes(item.Vector),
	}, ttl)
}

func (c *RedisSemanticCache) ensureIndex(ctx context.Context, dimension int) error {
	return c.store.CreateVectorIndex(ctx, VectorIndexSpec{
		Name:           semanticIndexName,
		Prefix:         semanticItemKeyPrefix,
		Dimension:      dimension,
		DistanceMetric: "COSINE",
	})
}

type RedisRateLimiter struct {
	store Store
}

func NewRedisRateLimiter(store Store) *RedisRateLimiter {
	return &RedisRateLimiter{store: store}
}

func (r *RedisRateLimiter) AllowUser(ctx context.Context, userID int64, limit int64, window time.Duration) (bool, error) {
	if r == nil || r.store == nil {
		return true, nil
	}
	count, err := r.store.IncrWithTTL(ctx, fmt.Sprintf("%s%d", rateKeyPrefix, userID), window)
	if err != nil {
		return false, err
	}
	return count <= limit, nil
}

type RedisCheckpointStore struct {
	store Store
	ttl   time.Duration
}

func NewRedisCheckpointStore(store Store, ttl time.Duration) *RedisCheckpointStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &RedisCheckpointStore{store: store, ttl: ttl}
}

func (s *RedisCheckpointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	if s == nil || s.store == nil {
		return nil, false, nil
	}
	data, err := s.store.Get(ctx, checkpointKeyPrefix+checkPointID)
	if err != nil || len(data) == 0 {
		return nil, false, err
	}
	return data, true, nil
}

func (s *RedisCheckpointStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Set(ctx, checkpointKeyPrefix+checkPointID, checkPoint, s.ttl)
}

var _ compose.CheckPointStore = (*RedisCheckpointStore)(nil)

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
	query = strings.ToLower(strings.TrimSpace(query))
	query = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case unicode.IsPunct(r), unicode.IsSymbol(r):
			return ' '
		default:
			return r
		}
	}, query)
	return strings.Join(strings.Fields(query), " ")
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

func buildSemanticQuery(req SemanticCacheLookup) string {
	scope := escapeTagValue(string(req.Scope))
	tenantID := escapeTagValue(req.TenantID)
	intentBucket := escapeTagValue(req.IntentBucket)
	if intentBucket == "" {
		intentBucket = "fallback"
	}
	filters := []string{
		fmt.Sprintf("@tenant:{%s}", tenantID),
		fmt.Sprintf("@scope:{%s}", scope),
		fmt.Sprintf("@intent_bucket:{%s}", intentBucket),
		fmt.Sprintf("@user_id:[%d %d]", scopedUserID(req.Scope, req.UserID), scopedUserID(req.Scope, req.UserID)),
	}
	return fmt.Sprintf("(%s)=>[KNN %d @embedding $vector AS vector_distance]", strings.Join(filters, " "), req.Limit)
}

func escapeTagValue(value string) string {
	replacer := strings.NewReplacer(
		"-", "\\-",
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"|", "\\|",
		" ", "\\ ",
		",", "\\,",
		".", "\\.",
		":", "\\:",
		";", "\\;",
		"!", "\\!",
		"@", "\\@",
		"#", "\\#",
		"$", "\\$",
		"%", "\\%",
		"^", "\\^",
		"&", "\\&",
		"*", "\\*",
		"~", "\\~",
		"'", "\\'",
		"\"", "\\\"",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func vectorBytes(vector []float64) []byte {
	buf := make([]byte, len(vector)*8)
	for i, value := range vector {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(value))
	}
	return buf
}

type semanticSearchHit struct {
	Item       *SemanticCacheItem
	Similarity float64
}

func parseSearchResults(raw any) ([]*semanticSearchHit, error) {
	values, ok := raw.([]any)
	if !ok || len(values) < 3 {
		return nil, nil
	}
	items := make([]*semanticSearchHit, 0, (len(values)-1)/2)
	for i := 1; i+1 < len(values); i += 2 {
		key := asString(values[i])
		fields, ok := values[i+1].([]any)
		if !ok {
			continue
		}
		item, similarity, err := semanticItemFromFields(key, fields)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, &semanticSearchHit{
				Item:       item,
				Similarity: similarity,
			})
		}
	}
	return items, nil
}

func semanticItemFromFields(key string, fields []any) (*SemanticCacheItem, float64, error) {
	values := make(map[string]string, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		values[asString(fields[i])] = asString(fields[i+1])
	}

	var response domain.ChatResult
	if raw := values["response"]; strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &response); err != nil {
			return nil, 0, err
		}
	}

	userID, _ := strconv.ParseInt(values["user_id"], 10, 64)
	createdAtMillis, _ := strconv.ParseInt(values["created_at"], 10, 64)
	similarity := similarityFromDistance(values["vector_distance"])

	item := &SemanticCacheItem{
		ID:           strings.TrimPrefix(key, semanticItemKeyPrefix),
		TenantID:     values["tenant"],
		UserID:       userID,
		IntentBucket: values["intent_bucket"],
		Scope:        CacheScope(values["scope"]),
		Query:        values["query"],
		Response:     response,
		CreatedAt:    time.UnixMilli(createdAtMillis),
		Vector:       nil,
	}
	item.Response.Confidence = maxFloat(item.Response.Confidence, similarity)
	return item, similarity, nil
}

func parseRedisFloat(value string) float64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	result, _ := strconv.ParseFloat(value, 64)
	return result
}

func similarityFromDistance(value string) float64 {
	distance := parseRedisFloat(value)
	similarity := 1 - distance
	if similarity < 0 {
		return 0
	}
	if similarity > 1 {
		return 1
	}
	return similarity
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func maxFloat(values ...float64) float64 {
	var max float64
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
