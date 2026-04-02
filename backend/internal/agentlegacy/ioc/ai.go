//go:build legacy_agent

package ioc

import (
	"time"

	"github.com/redis/go-redis/v9"

	infraai "github.com/XDWow/DouyinMall/backend/internal/agentlegacy/infra/ai"
	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"github.com/spf13/viper"
)

// InitFallbackLLMClient 鏋勫缓甯﹀閿欓摼鐨?LLM 瀹㈡埛绔?
//
// nodes 鍒囩墖鎸変紭鍏堢骇鎺掑垪锛氫富妯″瀷 鈫?澶囩敤妯″瀷锛堝彲閰嶇疆浠绘剰鏁伴噺锛夆啋 妯℃澘鍏滃簳
// 瓒呮椂鐢?openaiClient 鐨?http.Client.Timeout 鎺у埗锛孯esilientClient 鍙鐔旀柇 + 闄愭祦
//
// 渚濊禆 viper key锛?
//
//	llm.base_url / llm.api_key
//	llm.nodes: 鑺傜偣鍒楄〃锛屾瘡椤瑰寘鍚細
//	  name              妯″瀷鍚嶇О
//	  timeout_seconds   http 瓒呮椂锛堥粯璁?30s锛?
//	  rpm               RPM 闄愬埗锛堥粯璁?60锛?
//	  failure_threshold 鐔旀柇闃堝€硷紙榛樿 5锛?
//	  cooldown_seconds  鐔旀柇鍐峰嵈锛堥粯璁?30s锛?
func InitFallbackLLMClient(log logger.LoggerV1, metrics *usecase.PipelineMetrics, cmd redis.Cmdable) *infraai.FallbackLLMClient {
	baseURL := viper.GetString("llm.base_url")
	apiKey := viper.GetString("llm.api_key")

	type nodeCfg struct {
		Name             string
		TimeoutSeconds   int
		RPM              int
		FailureThreshold int32
		CooldownSeconds  int
	}
	var cfgs []nodeCfg
	if err := viper.UnmarshalKey("llm.nodes", &cfgs); err != nil || len(cfgs) == 0 {
		// 鍏煎鏃ч厤缃細llm.chat_model 鍗曡妭鐐?+ 鍙€?llm.fallback_model
		cfgs = []nodeCfg{
			{
				Name:             viper.GetString("llm.chat_model"),
				TimeoutSeconds:   viper.GetInt("llm.timeout_seconds"),
				RPM:              viper.GetInt("llm.rpm"),
				FailureThreshold: int32(viper.GetInt("llm.failure_threshold")),
				CooldownSeconds:  viper.GetInt("llm.cooldown_seconds"),
			},
		}
		if fm := viper.GetString("llm.fallback_model"); fm != "" {
			cfgs = append(cfgs, nodeCfg{
				Name:             fm,
				TimeoutSeconds:   viper.GetInt("llm.fallback_timeout_sec"),
				RPM:              viper.GetInt("llm.fallback_rpm"),
				FailureThreshold: int32(viper.GetInt("llm.failure_threshold")),
				CooldownSeconds:  viper.GetInt("llm.cooldown_seconds"),
			})
		}
	}

	log.Info("LLM 瀹归敊閾惧垵濮嬪寲",
		logger.String("base_url", baseURL),
		logger.Int("node_count", len(cfgs)))
	nodes := make([]infraai.CSLLMClient, 0, len(cfgs))
	for _, c := range cfgs {
		timeout := time.Duration(c.TimeoutSeconds) * time.Second
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		cooldown := time.Duration(c.CooldownSeconds) * time.Second
		if cooldown == 0 {
			cooldown = 30 * time.Second
		}
		threshold := c.FailureThreshold
		if threshold == 0 {
			threshold = 5
		}
		rpm := c.RPM
		if rpm == 0 {
			rpm = 60
		}
		// 姣忎釜鑺傜偣鐙珛鐨?Redis 婊戝姩绐楀彛闄愭祦鍣紝澶氬疄渚嬪叡浜鏁?
		log.Info("LLM 鑺傜偣娉ㄥ唽",
			logger.String("model", c.Name),
			logger.String("base_url", baseURL),
			logger.Int("timeout_s", c.TimeoutSeconds),
			logger.Int("rpm", rpm))
		nodeLimiter := ratelimit.NewRedisSlidingWindowLimiter(cmd, time.Minute, rpm)
		nodes = append(nodes, infraai.NewResilientClient(infraai.ResilientConfig{
			Inner: ai.NewOpenAIClient(ai.ChatConfig{
				BaseURL: baseURL,
				APIKey:  apiKey,
				Timeout: timeout,
			}),
			Limiter:          nodeLimiter,
			LimitKey:         "llm:rpm:" + c.Name,
			FailureThreshold: threshold,
			Cooldown:         cooldown,
			Model:            c.Name,
			Temperature:      0.7,
			MaxTokens:        512,
		}))
	}

	return infraai.NewFallbackLLMClient(log, metrics, nodes...)
}

// InitEmbedder 鍒濆鍖?Embedding 瀹㈡埛绔紙璇箟缂撳瓨 + RAG 鍚戦噺鍖栵級
func InitEmbedder() ai.Embedder {
	return ai.NewEmbeddingClient(ai.EmbeddingConfig{
		BaseURL: viper.GetString("embedding.base_url"),
		Model:   viper.GetString("embedding.model"),
		Timeout: time.Duration(viper.GetInt("embedding.timeout_seconds")) * time.Second,
	})
}

// InitReranker 鍒濆鍖?cross-encoder 閲嶆帓瀹㈡埛绔?
// 鍏煎 SiliconFlow / Jina 鐨?/rerank API锛堟帹鑽愭ā鍨嬶細BAAI/bge-reranker-v2-m3锛?
// 渚濊禆 viper key锛歳eranker.base_url / reranker.api_key / reranker.model / reranker.timeout_seconds
func InitReranker() ai.Reranker {
	return ai.NewRerankClient(ai.RerankConfig{
		BaseURL: viper.GetString("reranker.base_url"),
		APIKey:  viper.GetString("reranker.api_key"),
		Model:   viper.GetString("reranker.model"),
		Timeout: time.Duration(viper.GetInt("reranker.timeout_seconds")) * time.Second,
	})
}
