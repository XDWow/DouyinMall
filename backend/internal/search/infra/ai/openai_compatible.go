package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ==================== ChatClient (implements LLMClient) ====================

// ChatClient OpenAI 兼容的对话客户端
type ChatClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type ChatConfig struct {
	BaseURL string
	APIKey  string
	Model   string // 如 Qwen/Qwen2.5-7B-Instruct
	Timeout time.Duration
}

func NewChatClient(cfg ChatConfig) *ChatClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &ChatClient{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *ChatClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{"role": m.Role, "content": m.Content}
	}

	body := map[string]interface{}{
		"model":    c.model,
		"messages": messages,
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	respBody, err := doHTTPRequest(ctx, c.httpClient, c.baseURL+"/chat/completions", c.apiKey, body)
	if err != nil {
		return nil, fmt.Errorf("chat completion 失败: %w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回空结果")
	}

	return &ChatResponse{
		Content:    result.Choices[0].Message.Content,
		TokensUsed: result.Usage.TotalTokens,
	}, nil
}

// ==================== EmbeddingClient (implements Embedder) ====================

// EmbeddingClient OpenAI 兼容的 Embedding 客户端
type EmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type EmbeddingConfig struct {
	BaseURL string
	APIKey  string
	Model   string // 如 BAAI/bge-large-zh-v1.5
	Timeout time.Duration
}

func NewEmbeddingClient(cfg EmbeddingConfig) *EmbeddingClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &EmbeddingClient{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body := map[string]interface{}{
		"model": c.model,
		"input": texts,
	}

	respBody, err := doHTTPRequest(ctx, c.httpClient, c.baseURL+"/embeddings", c.apiKey, body)
	if err != nil {
		return nil, fmt.Errorf("embedding 失败: %w", err)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	vectors := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		vectors[i] = d.Embedding
	}
	return vectors, nil
}

// ==================== 公共 HTTP 工具 ====================

func doHTTPRequest(ctx context.Context, client *http.Client, url, apiKey string, body interface{}) ([]byte, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
