package ai

import (
	"context"
	"time"

	openaiembedder "github.com/cloudwego/eino-ext/components/embedding/openai"
	openaichat "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
)

type EinoChatModelConfig struct {
	Provider    string
	BaseURL     string
	APIKey      string
	Model       string
	Timeout     time.Duration
	Temperature float32
	MaxTokens   int
}

type EinoEmbeddingConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
}

func NewEinoChatModel(ctx context.Context, cfg EinoChatModelConfig) (model.ToolCallingChatModel, error) {
	return openaichat.NewChatModel(ctx, &openaichat.ChatModelConfig{
		BaseURL:     ResolveOpenAIBaseURL(NormalizeProvider(cfg.Provider), cfg.BaseURL),
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		Timeout:     cfg.Timeout,
		Temperature: float32Ptr(cfg.Temperature),
		MaxTokens:   intPtr(cfg.MaxTokens),
	})
}

func NewEinoEmbedder(ctx context.Context, cfg EinoEmbeddingConfig) (embedding.Embedder, error) {
	return openaiembedder.NewEmbedder(ctx, &openaiembedder.EmbeddingConfig{
		BaseURL: ResolveOpenAIBaseURL(NormalizeProvider(cfg.Provider), cfg.BaseURL),
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		Timeout: cfg.Timeout,
	})
}

func intPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func float32Ptr(value float32) *float32 {
	if value == 0 {
		return nil
	}
	return &value
}
