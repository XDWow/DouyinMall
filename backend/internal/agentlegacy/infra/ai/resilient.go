//go:build legacy_agent

package ai

import (
	"context"
	"errors"
	"time"

	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
)

var (
	ErrCircuitOpen = errors.New("鐔旀柇鍣ㄦ墦寮€")
	ErrRateLimited = errors.New("LLM 鑺傜偣璇锋眰瓒呭嚭闄愭祦")
)

// ResilientClient 鍗曡妭鐐瑰脊鎬ц楗板櫒锛氱啍鏂?+ 闄愭祦锛圧edis 婊戝姩绐楀彛锛?
// 瓒呮椂鐢辫皟鐢ㄦ柟 ctx 鎺у埗锛坕oc 灞傚湪鍒涘缓 openaiClient 鏃跺凡閰嶅ソ http.Client.Timeout锛?
// 瀹炵幇 CSLLMClient 鎺ュ彛
type ResilientClient struct {
	inner    pkgai.LLMClient
	breaker  *CircuitBreaker
	limiter  ratelimit.Limiter // Redis 婊戝姩绐楀彛锛屽瀹炰緥鍏变韩璁℃暟
	limitKey string            // Redis key锛岀敤妯″瀷鍚嶇О鍖哄垎涓嶅悓鑺傜偣

	// 妯″瀷鍙傛暟锛堜笟鍔″眰涓嶉渶瑕佸叧蹇冿級
	model       string
	temperature float32
	maxTokens   int
}

type ResilientConfig struct {
	Inner    pkgai.LLMClient
	Limiter  ratelimit.Limiter // 鐢?ioc 灞傚垱寤哄苟浼犲叆
	LimitKey string            // 妯″瀷鍚嶇О浣滀负 key锛屽疄鐜版ā鍨嬭妭鐐归檺娴?

	// 鐔旀柇
	FailureThreshold int32
	Cooldown         time.Duration

	// 妯″瀷鍙傛暟锛堜笟鍔″眰涓嶉渶瑕佸叧蹇冿級
	Model       string
	Temperature float32
	MaxTokens   int
}

func NewResilientClient(cfg ResilientConfig) *ResilientClient {
	return &ResilientClient{
		inner:       cfg.Inner,
		breaker:     NewCircuitBreaker(cfg.FailureThreshold, cfg.Cooldown),
		limiter:     cfg.Limiter,
		limitKey:    cfg.LimitKey,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
	}
}

func (r *ResilientClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if !r.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	// Redis 鏁呴殰鏃朵笉闃绘柇璇锋眰锛岄檷绾ф斁琛?
	if limited, err := r.limiter.Limit(ctx, r.limitKey); err == nil && limited {
		return nil, ErrRateLimited
	}

	// 琛ュ厖妯″瀷鍙傛暟
	pkgResp, err := r.inner.ChatCompletion(ctx, pkgai.ChatRequest{
		Model:       r.model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		Temperature: &r.temperature,
		MaxTokens:   &r.maxTokens,
	})
	if err != nil {
		r.breaker.RecordFailure()
		return nil, err
	}
	r.breaker.RecordSuccess()

	return &ChatResponse{
		ID:         pkgResp.ID,
		Created:    pkgResp.Created,
		Choices:    pkgResp.Choices,
		TokensUsed: pkgResp.Usage.TotalTokens,
	}, nil
}

func (r *ResilientClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	if !r.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	// Redis 鏁呴殰鏃朵笉闃绘柇璇锋眰锛岄檷绾ф斁琛?
	if limited, err := r.limiter.Limit(ctx, r.limitKey); err == nil && limited {
		return nil, ErrRateLimited
	}

	// 琛ュ厖妯″瀷鍙傛暟
	pkgCh, err := r.inner.ChatCompletionStream(ctx, pkgai.ChatRequest{
		Model:       r.model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		Temperature: &r.temperature,
		MaxTokens:   &r.maxTokens,
	})
	if err != nil {
		r.breaker.RecordFailure()
		return nil, err
	}
	r.breaker.RecordSuccess()

	// 濡傛灉鏄伐鍏疯皟鐢ㄥ弬鏁帮紝绱Н锛歄penAI 娴佸紡杩斿洖鏃?arguments 浼氬垎澶氫釜 chunk
	// 绱Н瀹屾垚鍚庯紙finish_reason=tool_calls锛夋墠鍙戦€佺粰涓婂眰锛屼笂灞傛棤鎰熺煡锛岀洿鎺ュ鐞?
	ch := make(chan ChatResponse, 32)
	go func() {
		defer close(ch)

		// 绱Н鐘舵€侊細index -> ToolCall
		toolCallsMap := make(map[int]*pkgai.ToolCall)
		var fullContent string

		for chunk := range pkgCh {
			if len(chunk.Choices) == 0 {
				continue
			}
			choice := chunk.Choices[0]

			// 涓€涓?chunk 鍙仛涓€浠朵簨锛氳涔堢敓鎴愭枃鏈紝瑕佷箞鐢熸垚宸ュ叿鍙傛暟
			// 濡傛灉鏈夋枃鏈閲忥紝绔嬪嵆鎺ㄩ€?
			if choice.Delta.Content != "" {
				fullContent += choice.Delta.Content
				ch <- ChatResponse{
					ID:      chunk.ID,
					Created: chunk.Created,
					Choices: []pkgai.Choice{{
						Index: 0,
						Message: pkgai.Message{
							Role:    "assistant",
							Content: choice.Delta.Content,
						},
					}},
				}
				continue // 杩欎釜 chunk 鍙湁鏂囨湰锛岃烦杩囧伐鍏峰弬鏁板鐞?
			}

			// 濡傛灉娌℃湁鏂囨湰锛屽氨绱Н宸ュ叿鍙傛暟
			for _, tc := range choice.ToolCalls {
				if existing, ok := toolCallsMap[tc.Index]; !ok {
					// 棣栨鍑虹幇璇?index锛屽垵濮嬪寲
					toolCallsMap[tc.Index] = &pkgai.ToolCall{
						Index: tc.Index,
						ID:    tc.ID,
						Type:  tc.Type,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				} else {
					// 绱Н arguments锛堝垎鐗囧埌杈撅級
					existing.Function.Arguments += tc.Function.Arguments
					if existing.ID == "" && tc.ID != "" {
						existing.ID = tc.ID
					}
					if existing.Type == "" && tc.Type != "" {
						existing.Type = tc.Type
					}
					if existing.Function.Name == "" && tc.Function.Name != "" {
						existing.Function.Name = tc.Function.Name
					}
				}
			}

			// finish_reason=tool_calls 浠ｈ〃鍙傛暟绱Н瀹屾垚锛屽彂閫佺粰涓婂眰
			if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
				toolCalls := make([]pkgai.ToolCall, 0, len(toolCallsMap))
				for i := 0; i < len(toolCallsMap); i++ {
					if tc, ok := toolCallsMap[i]; ok {
						toolCalls = append(toolCalls, *tc)
					}
				}

				ch <- ChatResponse{
					ID:      chunk.ID,
					Created: chunk.Created,
					Choices: []pkgai.Choice{{
						Index:        0,
						FinishReason: choice.FinishReason,
						Message: pkgai.Message{
							Role:      "assistant",
							Content:   fullContent,
							ToolCalls: toolCalls,
						},
					}},
				}
				return
			}
		}
	}()

	return ch, nil
}
