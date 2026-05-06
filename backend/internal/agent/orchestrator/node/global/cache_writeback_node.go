package global

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type CacheWritebackService struct {
	ExactCache       cache.ExactCache
	SemanticCache    cache.SemanticCache
	Embedder         embedding.Embedder
	ExactCacheTTL    time.Duration
	SemanticCacheTTL time.Duration
	Logger           logger.LoggerV1
}

func NewCacheWritebackService(
	exactCache cache.ExactCache,
	semanticCache cache.SemanticCache,
	embedder embedding.Embedder,
	exactCacheTTL time.Duration,
	semanticCacheTTL time.Duration,
	log logger.LoggerV1,
) *CacheWritebackService {
	return &CacheWritebackService{
		ExactCache:       exactCache,
		SemanticCache:    semanticCache,
		Embedder:         embedder,
		ExactCacheTTL:    exactCacheTTL,
		SemanticCacheTTL: semanticCacheTTL,
		Logger:           log,
	}
}

func (n *CacheWritebackService) Write(ctx context.Context, state *domain.State) error {
	if n == nil || state == nil || state.Input == nil || state.Response == nil {
		return nil
	}
	if !CanWriteCache(state) {
		return nil
	}

	query := strings.TrimSpace(state.Input.Message)
	if query == "" {
		return nil
	}

	resp := cloneCacheableResponse(state.Response)
	if n.ExactCache != nil {
		if err := n.ExactCache.Store(ctx, &cache.ExactCacheItem{
			TenantID:  TenantIDOf(state, "default"),
			UserID:    state.Input.UserID,
			Query:     query,
			Response:  resp,
			CreatedAt: time.Now(),
		}, n.ExactCacheTTL); err != nil {
			return err
		}
	}

	semanticQuery := strings.TrimSpace(state.RewrittenQuery)
	if semanticQuery == "" {
		semanticQuery = query
	}
	if !semanticCacheableIntent(state.Intent) || n.SemanticCache == nil || n.Embedder == nil || semanticQuery == "" {
		return nil
	}

	vectors, err := n.Embedder.EmbedStrings(ctx, []string{semanticQuery})
	if err != nil {
		return err
	}
	if len(vectors) == 0 {
		return nil
	}

	return n.SemanticCache.Store(ctx, &cache.SemanticCacheItem{
		TenantID:     TenantIDOf(state, "default"),
		UserID:       state.Input.UserID,
		IntentBucket: semanticIntentBucket(state.Intent),
		Scope:        semanticScopeForIntent(state.Intent),
		Query:        semanticQuery,
		Response:     resp,
		Vector:       vectors[0],
		CreatedAt:    time.Now(),
	}, n.SemanticCacheTTL)
}

func cloneCacheableResponse(resp *domain.ChatResult) domain.ChatResult {
	if resp == nil {
		return domain.ChatResult{}
	}
	out := *resp
	out.SessionID = ""
	out.TraceID = ""
	out.Interrupt = nil
	out.Interrupted = false
	out.Streamed = false
	out.Trace = domain.Trace{}
	out.ToolExecutions = append([]domain.ToolExecution(nil), resp.ToolExecutions...)
	out.UsedToolNames = append([]string(nil), resp.UsedToolNames...)
	out.References = append([]domain.KnowledgeRef(nil), resp.References...)
	return out
}
