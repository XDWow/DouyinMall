package ai

import (
	"context"
	"errors"
	"time"

	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
)

var (
	ErrCircuitOpen = errors.New("熔断器打开")
	ErrRateLimited = errors.New("LLM 节点请求超出限流")
)

// ResilientClient 单节点弹性装饰器：熔断 + 限流（Redis 滑动窗口）
// 超时由调用方 ctx 控制（ioc 层在创建 openaiClient 时已配好 http.Client.Timeout）
// 实现 CSLLMClient 接口
type ResilientClient struct {
	inner    pkgai.LLMClient
	breaker  *CircuitBreaker
	limiter  ratelimit.Limiter // Redis 滑动窗口，多实例共享计数
	limitKey string            // Redis key，用模型名称区分不同节点

	// 模型参数（业务层不需要关心）
	model       string
	temperature float32
	maxTokens   int
}

type ResilientConfig struct {
	Inner    pkgai.LLMClient
	Limiter  ratelimit.Limiter // 由 ioc 层创建并传入
	LimitKey string            // 模型名称作为 key，实现模型节点限流

	// 熔断
	FailureThreshold int32
	Cooldown         time.Duration

	// 模型参数（业务层不需要关心）
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
	// Redis 故障时不阻断请求，降级放行
	if limited, err := r.limiter.Limit(ctx, r.limitKey); err == nil && limited {
		return nil, ErrRateLimited
	}

	// 补充模型参数
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
	// Redis 故障时不阻断请求，降级放行
	if limited, err := r.limiter.Limit(ctx, r.limitKey); err == nil && limited {
		return nil, ErrRateLimited
	}

	// 补充模型参数
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

	// 如果是工具调用参数，累积：OpenAI 流式返回时 arguments 会分多个 chunk
	// 累积完成后（finish_reason=tool_calls）才发送给上层，上层无感知，直接处理
	ch := make(chan ChatResponse, 32)
	go func() {
		defer close(ch)

		// 累积状态：index -> ToolCall
		toolCallsMap := make(map[int]*pkgai.ToolCall)
		var fullContent string

		for chunk := range pkgCh {
			if len(chunk.Choices) == 0 {
				continue
			}
			choice := chunk.Choices[0]

			// 一个 chunk 只做一件事：要么生成文本，要么生成工具参数
			// 如果有文本增量，立即推送
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
				continue // 这个 chunk 只有文本，跳过工具参数处理
			}

			// 如果没有文本，就累积工具参数
			for _, tc := range choice.ToolCalls {
				if existing, ok := toolCallsMap[tc.Index]; !ok {
					// 首次出现该 index，初始化
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
					// 累积 arguments（分片到达）
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

			// finish_reason=tool_calls 代表参数累积完成，发送给上层
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
