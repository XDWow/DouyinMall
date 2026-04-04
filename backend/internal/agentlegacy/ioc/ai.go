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

// InitFallbackLLMClient 閺嬪嫬缂撶敮锕€顔愰柨娆撴懠閻?LLM 鐎广垺鍩涚粩?
//
// nodes 閸掑洨澧栭幐澶夌喘閸忓牏楠囬幒鎺戝灙閿涙矮瀵屽Ο鈥崇€?閳?婢跺洨鏁ゅΟ鈥崇€烽敍鍫濆讲闁板秶鐤嗘禒缁樺壈閺佷即鍣洪敍澶嗗晪 濡剝婢橀崗婊冪俺
// 鐡掑懏妞傞悽?openaiClient 閻?http.Client.Timeout 閹貉冨煑閿涘esilientClient 閸欘亞顓搁悢鏃€鏌?+ 闂勬劖绁?//
// 娓氭繆绂?viper key閿?
//
//	llm.base_url / llm.api_key
//	llm.nodes: 閼哄倻鍋ｉ崚妤勩€冮敍灞剧槨妞ょ懓瀵橀崥顐窗
//	  name              濡€崇€烽崥宥囆?//	  timeout_seconds   http 鐡掑懏妞傞敍鍫ョ帛鐠?30s閿?
//	  rpm               RPM 闂勬劕鍩楅敍鍫ョ帛鐠?60閿?
//	  failure_threshold 閻旀梹鏌囬梼鍫濃偓纭风礄姒涙顓?5閿?
//	  cooldown_seconds  閻旀梹鏌囬崘宄板祱閿涘牓绮拋?30s閿?
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
		// 閸忕厧顔愰弮褔鍘ょ純顕嗙窗llm.chat_model 閸楁洝濡悙?+ 閸欘垶鈧?llm.fallback_model
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

	log.Info("LLM 鐎瑰綊鏁婇柧鎯у灥婵瀵?,
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
		// 濮ｅ繋閲滈懞鍌滃仯閻欘剛鐝涢惃?Redis 濠婃垵濮╃粣妤€褰涢梽鎰ウ閸ｎ煉绱濇径姘杽娓氬鍙℃禍顐ヮ吀閺?
		log.Info("LLM 閼哄倻鍋ｅ▔銊ュ斀",
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

// InitEmbedder 閸掓繂顫愰崠?Embedding 鐎广垺鍩涚粩顖ょ礄鐠囶厺绠熺紓鎾崇摠 + RAG 閸氭垿鍣洪崠鏍电礆
func InitEmbedder() ai.Embedder {
	return ai.NewEmbeddingClient(ai.EmbeddingConfig{
		BaseURL: viper.GetString("embedding.base_url"),
		Model:   viper.GetString("embedding.model"),
		Timeout: time.Duration(viper.GetInt("embedding.timeout_seconds")) * time.Second,
	})
}

// InitReranker 閸掓繂顫愰崠?cross-encoder 闁插秵甯撶€广垺鍩涚粩?
// 閸忕厧顔?SiliconFlow / Jina 閻?/rerank API閿涘牊甯归懡鎰侀崹瀣剁窗BAAI/bge-reranker-v2-m3閿?
// 娓氭繆绂?viper key閿涙eranker.base_url / reranker.api_key / reranker.model / reranker.timeout_seconds
func InitReranker() ai.Reranker {
	return ai.NewRerankClient(ai.RerankConfig{
		BaseURL: viper.GetString("reranker.base_url"),
		APIKey:  viper.GetString("reranker.api_key"),
		Model:   viper.GetString("reranker.model"),
		Timeout: time.Duration(viper.GetInt("reranker.timeout_seconds")) * time.Second,
	})
}


