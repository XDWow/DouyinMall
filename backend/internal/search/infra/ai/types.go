package ai

import "context"

// LLMClient 大模型对话接口（Query 理解、RAG 摘要）
type LLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Embedder 文本向量化接口（与 LLMClient 职责不同：text → dense vector）
// Embedding 模型（如 bge-large-zh）与对话模型（如 Qwen）通常不同，独立接口可分别替换
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// ChatRequest 对话请求
type ChatRequest struct {
	Messages    []Message
	Temperature float32 // 0~1，越低越确定
	MaxTokens   int
}

// Message 对话消息
type Message struct {
	// system：定义AI行为规则
	// user：当前用户输入
	// assistant：历史AI输出，用于维持上下文连续性
	Role    string
	Content string
}

// ChatResponse 对话响应
type ChatResponse struct {
	Content    string
	TokensUsed int
}
