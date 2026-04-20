package llm

import (
	"context"
	"time"

	openaichat "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
)

type ChatModelConfig struct {
	Provider    string
	BaseURL     string
	APIKey      string
	Model       string
	Timeout     time.Duration
	Temperature float32
	MaxTokens   int
}

func NewChatModel(ctx context.Context, cfg ChatModelConfig) (model.ToolCallingChatModel, error) {
	return openaichat.NewChatModel(ctx, &openaichat.ChatModelConfig{
		BaseURL:     pkgai.ResolveOpenAIBaseURL(pkgai.NormalizeProvider(cfg.Provider), cfg.BaseURL),
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		Timeout:     cfg.Timeout,
		Temperature: float32Ptr(cfg.Temperature),
		MaxTokens:   intPtr(cfg.MaxTokens),
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
