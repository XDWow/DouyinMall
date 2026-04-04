package ioc

import (
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/spf13/viper"
)

// InitLLMClient 鍒濆鍖?LLM 瀵硅瘽瀹㈡埛绔紙Query 鐞嗚В銆丷AG 鎽樿锛?
func InitLLMClient() ai.LLMClient {
	return ai.NewOpenAIClient(ai.ChatConfig{
		BaseURL: viper.GetString("llm.base_url"),
		APIKey:  viper.GetString("llm.api_key"),
		Timeout: time.Duration(viper.GetInt("llm.timeout_seconds")) * time.Second,
	})
}

// InitEmbedder 鍒濆鍖?Embedding 瀹㈡埛绔紙鏂囨湰鈫掑悜閲忥紝鐙珛浜?LLM锛?
func InitEmbedder() ai.Embedder {
	return ai.NewEmbeddingClient(ai.EmbeddingConfig{
		BaseURL: viper.GetString("embedding.base_url"),
		APIKey:  viper.GetString("embedding.api_key"),
		Model:   viper.GetString("embedding.model"),
		Timeout: time.Duration(viper.GetInt("embedding.timeout_seconds")) * time.Second,
	})
}


