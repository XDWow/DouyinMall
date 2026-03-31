package ai

import (
	"context"
	"encoding/json"
)

type LLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamResponse, error)
}

// StreamToken 流式响应的单个 token（已废弃，使用 ChatStreamResponse）
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

// ==================== 对接心流平台 API ====================

// ---------- 请求结构 -----------------
type ChatRequest struct {
	Messages         []Message       `json:"messages"`                    // 必填
	Model            string          `json:"model"`                       // 必填
	FrequencyPenalty *float32        `json:"frequency_penalty,omitempty"` // 默认 0.5
	MaxTokens        *int            `json:"max_tokens,omitempty"`        // 默认 512
	N                *int            `json:"n,omitempty"`                 // 默认 1
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`   // 可选
	Stop             []string        `json:"stop,omitempty"`
	Stream           bool            `json:"stream,omitempty"`      // 默认 false
	Temperature      *float32        `json:"temperature,omitempty"` // 默认 0.7
	Tools            []ToolDef       `json:"tools,omitempty"`
	TopK             *float32        `json:"top_k,omitempty"` // 默认 50
	TopP             *float32        `json:"top_p,omitempty"` // 默认 0.7
}

type Message struct {
	Role       string     `json:"role"`                   // system/user/assistant/tool
	Content    string     `json:"content"`                // 文本内容
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // 模型回复：role=assistant 时，LLM 返回的工具调用需求
	ToolCallID string     `json:"tool_call_id,omitempty"` // role=tool 时，发给 llm 的工具调用结果，并表明ID，对应某次tool调用
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
	Strict      *bool           `json:"strict,omitempty"` // 默认 false
}

// -------------------- 响应结构 ------------------------
// 非流式响应
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

// 流式响应
type ChatStreamResponse struct {
	ID      string                 `json:"id"`
	Model   string                 `json:"model"`
	Created int64                  `json:"created"`
	Object  string                 `json:"object"`
	Usage   *UsageObj              `json:"usage,omitempty"`
	Choices []ProviderStreamChoice `json:"choices"`

	// 暂时没用
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

// 工具调用定义
type ToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`   // 工具调用唯一标识
	Type     string `json:"type,omitempty"` // 枚举: function
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"` // JSON string
	} `json:"function"`
}
