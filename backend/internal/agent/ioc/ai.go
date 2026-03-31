//go:build legacy_agent

package ioc

import (
	"time"

	"github.com/redis/go-redis/v9"

	infraai "github.com/XDWow/DouyinMall/backend/internal/agent/infra/ai"
	"github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
	"github.com/spf13/viper"
)

// InitFallbackLLMClient 构建带容错链的 LLM 客户端
//
// nodes 切片按优先级排列：主模型 → 备用模型（可配置任意数量）→ 模板兜底
// 超时由 openaiClient 的 http.Client.Timeout 控制，ResilientClient 只管熔断 + 限流
//
// 依赖 viper key：
//
//	llm.base_url / llm.api_key
//	llm.nodes: 节点列表，每项包含：
//	  name              模型名称
//	  timeout_seconds   http 超时（默认 30s）
//	  rpm               RPM 限制（默认 60）
//	  failure_threshold 熔断阈值（默认 5）
//	  cooldown_seconds  熔断冷却（默认 30s）
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
		// 兼容旧配置：llm.chat_model 单节点 + 可选 llm.fallback_model
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

	log.Info("LLM 容错链初始化",
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
		// 每个节点独立的 Redis 滑动窗口限流器，多实例共享计数
		log.Info("LLM 节点注册",
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

// InitEmbedder 初始化 Embedding 客户端（语义缓存 + RAG 向量化）
func InitEmbedder() ai.Embedder {
	return ai.NewEmbeddingClient(ai.EmbeddingConfig{
		BaseURL: viper.GetString("embedding.base_url"),
		Model:   viper.GetString("embedding.model"),
		Timeout: time.Duration(viper.GetInt("embedding.timeout_seconds")) * time.Second,
	})
}

// InitReranker 初始化 cross-encoder 重排客户端
// 兼容 SiliconFlow / Jina 的 /rerank API（推荐模型：BAAI/bge-reranker-v2-m3）
// 依赖 viper key：reranker.base_url / reranker.api_key / reranker.model / reranker.timeout_seconds
func InitReranker() ai.Reranker {
	return ai.NewRerankClient(ai.RerankConfig{
		BaseURL: viper.GetString("reranker.base_url"),
		APIKey:  viper.GetString("reranker.api_key"),
		Model:   viper.GetString("reranker.model"),
		Timeout: time.Duration(viper.GetInt("reranker.timeout_seconds")) * time.Second,
	})
}
