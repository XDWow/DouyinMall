package llm

import (
	"context"
	"time"

	openaiembedder "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
)

type EmbeddingConfig struct {
	Provider string
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func NewEmbedder(ctx context.Context, cfg EmbeddingConfig) (embedding.Embedder, error) {
	return openaiembedder.NewEmbedder(ctx, &openaiembedder.EmbeddingConfig{
		BaseURL: pkgai.ResolveOpenAIBaseURL(pkgai.NormalizeProvider(cfg.Provider), cfg.BaseURL),
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		Timeout: cfg.Timeout,
	})
}
