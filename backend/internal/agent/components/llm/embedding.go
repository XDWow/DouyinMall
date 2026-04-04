package llm

import (
	"context"
	"time"

	openaiembedder "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
)

type EmbeddingConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func NewEmbedder(ctx context.Context, cfg EmbeddingConfig) (embedding.Embedder, error) {
	return openaiembedder.NewEmbedder(ctx, &openaiembedder.EmbeddingConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		Timeout: cfg.Timeout,
	})
}
