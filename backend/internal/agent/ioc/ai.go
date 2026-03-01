package ioc

import (
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/search/infra/ai"
	"github.com/spf13/viper"
)

// InitLLMClient 初始化 LLM 对话客户端（意图识别、Rerank、回复生成）
func InitLLMClient() ai.LLMClient {
	return ai.NewChatClient(ai.ChatConfig{
		BaseURL: viper.GetString("llm.base_url"),
		APIKey:  viper.GetString("llm.api_key"),
		Model:   viper.GetString("llm.chat_model"),
		Timeout: time.Duration(viper.GetInt("llm.timeout_seconds")) * time.Second,
	})
}

// InitEmbedder 初始化 Embedding 客户端（语义缓存 + RAG 向量化）
func InitEmbedder() ai.Embedder {
	return ai.NewEmbeddingClient(ai.EmbeddingConfig{
		BaseURL: viper.GetString("embedding.base_url"),
		APIKey:  viper.GetString("embedding.api_key"),
		Model:   viper.GetString("embedding.model"),
		Timeout: time.Duration(viper.GetInt("embedding.timeout_seconds")) * time.Second,
	})
}
