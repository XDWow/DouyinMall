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
	ErrCircuitOpen = errors.New("閻旀梹鏌囬崳銊﹀ⅵ瀵偓")
	ErrRateLimited = errors.New("LLM 閼哄倻鍋ｇ拠閿嬬湴鐡掑懎鍤梽鎰ウ")
)

// ResilientClient 閸楁洝濡悙鐟拌剨閹嗩棅妤楁澘娅掗敍姘卞晬閺?+ 闂勬劖绁﹂敍鍦dis 濠婃垵濮╃粣妤€褰涢敍?
// 鐡掑懏妞傞悽杈殶閻劍鏌?ctx 閹貉冨煑閿涘潟oc 鐏炲倸婀崚娑樼紦 openaiClient 閺冭泛鍑￠柊宥呫偨 http.Client.Timeout閿?
// 鐎圭偟骞?CSLLMClient 閹恒儱褰?type ResilientClient struct {
	inner    pkgai.LLMClient
	breaker  *CircuitBreaker
	limiter  ratelimit.Limiter // Redis 濠婃垵濮╃粣妤€褰涢敍灞筋樋鐎圭偘绶ラ崗鍙橀煩鐠佲剝鏆?	limitKey string            // Redis key閿涘瞼鏁ゅΟ鈥崇€烽崥宥囆為崠鍝勫瀻娑撳秴鎮撻懞鍌滃仯

	// 濡€崇€烽崣鍌涙殶閿涘牅绗熼崝鈥崇湴娑撳秹娓剁憰浣稿彠韫囧喛绱?	model       string
	temperature float32
	maxTokens   int
}

type ResilientConfig struct {
	Inner    pkgai.LLMClient
	Limiter  ratelimit.Limiter // 閻?ioc 鐏炲倸鍨卞鍝勮嫙娴肩姴鍙?	LimitKey string            // 濡€崇€烽崥宥囆炴担婊€璐?key閿涘苯鐤勯悳鐗埬侀崹瀣Ν閻愬綊妾哄ù?

	// 閻旀梹鏌?	FailureThreshold int32
	Cooldown         time.Duration

	// 濡€崇€烽崣鍌涙殶閿涘牅绗熼崝鈥崇湴娑撳秹娓剁憰浣稿彠韫囧喛绱?	Model       string
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
	// Redis 閺佸懘娈伴弮鏈电瑝闂冪粯鏌囩拠閿嬬湴閿涘矂妾风痪褎鏂佺悰?
	if limited, err := r.limiter.Limit(ctx, r.limitKey); err == nil && limited {
		return nil, ErrRateLimited
	}

	// 鐞涖儱鍘栧Ο鈥崇€烽崣鍌涙殶
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
	// Redis 閺佸懘娈伴弮鏈电瑝闂冪粯鏌囩拠閿嬬湴閿涘矂妾风痪褎鏂佺悰?
	if limited, err := r.limiter.Limit(ctx, r.limitKey); err == nil && limited {
		return nil, ErrRateLimited
	}

	// 鐞涖儱鍘栧Ο鈥崇€烽崣鍌涙殶
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

	// 婵″倹鐏夐弰顖氫紣閸忕柉鐨熼悽銊ュ棘閺佸府绱濈槐顖溞濋敍姝刾enAI 濞翠礁绱℃潻鏂挎礀閺?arguments 娴兼艾鍨庢径姘嚋 chunk
	// 缁鳖垳袧鐎瑰本鍨氶崥搴礄finish_reason=tool_calls閿涘澧犻崣鎴︹偓浣虹舶娑撳﹤鐪伴敍灞肩瑐鐏炲倹妫ら幇鐔虹叀閿涘瞼娲块幒銉ヮ槱閻?
	ch := make(chan ChatResponse, 32)
	go func() {
		defer close(ch)

		// 缁鳖垳袧閻樿埖鈧緤绱癷ndex -> ToolCall
		toolCallsMap := make(map[int]*pkgai.ToolCall)
		var fullContent string

		for chunk := range pkgCh {
			if len(chunk.Choices) == 0 {
				continue
			}
			choice := chunk.Choices[0]

			// 娑撯偓娑?chunk 閸欘亜浠涙稉鈧禒鏈电皑閿涙俺顩︽稊鍫㈡晸閹存劖鏋冮張顒婄礉鐟曚椒绠為悽鐔稿灇瀹搞儱鍙块崣鍌涙殶
			// 婵″倹鐏夐張澶嬫瀮閺堫剙顤冮柌蹇ョ礉缁斿宓嗛幒銊┾偓?
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
				continue // 鏉╂瑤閲?chunk 閸欘亝婀侀弬鍥ㄦ拱閿涘矁鐑︽潻鍥т紣閸忓嘲寮弫鏉款槱閻?
			}

			// 婵″倹鐏夊▽鈩冩箒閺傚洦婀伴敍灞芥皑缁鳖垳袧瀹搞儱鍙块崣鍌涙殶
			for _, tc := range choice.ToolCalls {
				if existing, ok := toolCallsMap[tc.Index]; !ok {
					// 妫ｆ牗顐奸崙铏瑰箛鐠?index閿涘苯鍨垫慨瀣
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
					// 缁鳖垳袧 arguments閿涘牆鍨庨悧鍥у煂鏉堟拝绱?					existing.Function.Arguments += tc.Function.Arguments
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

			// finish_reason=tool_calls 娴狅綀銆冮崣鍌涙殶缁鳖垳袧鐎瑰本鍨氶敍灞藉絺闁胶绮版稉濠傜湴
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


