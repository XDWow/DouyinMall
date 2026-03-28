package ioc

import (
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/spf13/viper"
)

// InitLLMClient 初始化 LLM 对话客户端（Query 理解、RAG 摘要）
func InitLLMClient() ai.LLMClient {
	return ai.NewOpenAIClient(ai.ChatConfig{
		BaseURL: viper.GetString("llm.base_url"),
		APIKey:  viper.GetString("llm.api_key"),
		Timeout: time.Duration(viper.GetInt("llm.timeout_seconds")) * time.Second,
	})
}

// InitEmbedder 初始化 Embedding 客户端（文本→向量，独立于 LLM）
func InitEmbedder() ai.Embedder {
	return ai.NewEmbeddingClient(ai.EmbeddingConfig{
		BaseURL: viper.GetString("embedding.base_url"),
		APIKey:  viper.GetString("embedding.api_key"),
		Model:   viper.GetString("embedding.model"),
		Timeout: time.Duration(viper.GetInt("embedding.timeout_seconds")) * time.Second,
	})
}
