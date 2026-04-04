package ai

import (
	"context"
	"encoding/json"
)

type LLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamResponse, error)
}

// StreamToken 娴佸紡鍝嶅簲鐨勫崟涓?token锛堝凡搴熷純锛屼娇鐢?ChatStreamResponse锛?
type StreamToken struct {
	Delta     string
	ToolCalls []ToolCall
	Err       error
}

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type Reranker interface {
	Rerank(ctx context.Context, query string, docs []string) ([]float32, error)
}

// ==================== 瀵规帴蹇冩祦骞冲彴 API ====================

// ---------- 璇锋眰缁撴瀯 -----------------
type ChatRequest struct {
	Messages         []Message       `json:"messages"`                    // 蹇呭～
	Model            string          `json:"model"`                       // 蹇呭～
	FrequencyPenalty *float32        `json:"frequency_penalty,omitempty"` // 榛樿 0.5
	MaxTokens        *int            `json:"max_tokens,omitempty"`        // 榛樿 512
	N                *int            `json:"n,omitempty"`                 // 榛樿 1
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`   // 鍙€?
	Stop             []string        `json:"stop,omitempty"`
	Stream           bool            `json:"stream,omitempty"`      // 榛樿 false
	Temperature      *float32        `json:"temperature,omitempty"` // 榛樿 0.7
	Tools            []ToolDef       `json:"tools,omitempty"`
	TopK             *float32        `json:"top_k,omitempty"` // 榛樿 50
	TopP             *float32        `json:"top_p,omitempty"` // 榛樿 0.7
}

type Message struct {
	Role       string     `json:"role"`                   // system/user/assistant/tool
	Content    string     `json:"content"`                // 鏂囨湰鍐呭
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // 妯″瀷鍥炲锛歳ole=assistant 鏃讹紝LLM 杩斿洖鐨勫伐鍏疯皟鐢ㄩ渶姹?
	ToolCallID string     `json:"tool_call_id,omitempty"` // role=tool 鏃讹紝鍙戠粰 llm 鐨勫伐鍏疯皟鐢ㄧ粨鏋滐紝骞惰〃鏄嶪D锛屽搴旀煇娆ool璋冪敤
}

type ResponseFormat struct {
	Type string `json:"type,omitempty"`
}

type ToolDef struct {
	Type     string      `json:"type"` // function
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`       // JSON Schema
	Strict      *bool           `json:"strict,omitempty"` // 榛樿 false
}

// -------------------- 鍝嶅簲缁撴瀯 ------------------------
// 闈炴祦寮忓搷搴?
type ChatResponse struct {
	ID      string   `json:"id"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Object  string   `json:"object"` // chat.completion
	Choices []Choice `json:"choices"`
	Usage   UsageObj `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason,omitempty"` // stop/eos/length/tool_calls
	Message      Message `json:"message"`
}

// 娴佸紡鍝嶅簲
type ChatStreamResponse struct {
	ID      string                 `json:"id"`
	Model   string                 `json:"model"`
	Created int64                  `json:"created"`
	Object  string                 `json:"object"`
	Usage   *UsageObj              `json:"usage,omitempty"`
	Choices []ProviderStreamChoice `json:"choices"`

	// 鏆傛椂娌＄敤
	ServiceTier       *string `json:"service_tier,omitempty"`
	SystemFingerprint *string `json:"system_fingerprint"`
}

type UsageObj struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ProviderStreamChoice struct {
	Index        int           `json:"index"`
	Delta        ProviderDelta `json:"delta,omitempty"`
	FinishReason *string       `json:"finish_reason,omitempty"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
}

type ProviderDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// 宸ュ叿璋冪敤瀹氫箟
type ToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`   // 宸ュ叿璋冪敤鍞竴鏍囪瘑
	Type     string `json:"type,omitempty"` // 鏋氫妇: function
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"` // JSON string
	} `json:"function"`
}


