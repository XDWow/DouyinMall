package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type llmClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type ChatConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

func NewOpenAIClient(cfg ChatConfig) LLMClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &llmClient{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *llmClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req = c.buildRequestBody(req)

	respBody, err := doHTTPRequest(ctx, c.httpClient, c.baseURL+"/chat/completions", c.apiKey, req)
	if err != nil {
		return nil, fmt.Errorf("chat completion 失败: %w", err)
	}

	var result ChatResponse
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		err = errors.New("模型回复内容为空")
	}

	return &result, err
}

// 流式对话（SSE）
func (c *llmClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamResponse, error) {
	req = c.buildRequestBody(req)

	bodyJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream") // 选择 sse 协议
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stream request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	ch := make(chan ChatStreamResponse, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk ChatStreamResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 && chunk.Usage == nil {
				continue
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// 请求构建，给默认值
func (c *llmClient) buildRequestBody(req ChatRequest) ChatRequest {
	if req.FrequencyPenalty == nil {
		f := float32(0.5)
		req.FrequencyPenalty = &f
	}
	if req.MaxTokens == nil {
		m := 512
		req.MaxTokens = &m
	}
	if req.N == nil {
		n := 1
		req.N = &n
	}
	if req.Temperature == nil {
		t := float32(0.7)
		req.Temperature = &t
	}
	if req.TopK == nil {
		k := float32(50)
		req.TopK = &k
	}
	if req.TopP == nil {
		p := float32(0.7)
		req.TopP = &p
	}

	return req
}

// 本地 ollama 实现
type EmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type EmbeddingConfig struct {
	BaseURL string
	APIKey  string
	Model   string
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

	respBody, err := doHTTPRequest(ctx, c.httpClient, c.baseURL, c.apiKey, body)
	if err != nil {
		return nil, fmt.Errorf("embedding 失败: %w", err)
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result.Embeddings, nil
}

// RerankClient OpenAI 兼容的 Rerank 客户端
// 兼容 SiliconFlow / Jina / Cohere 的 /rerank API
// 请求格式：POST /rerank {"model":"...","query":"...","documents":["doc1","doc2",...]}
// 响应格式：{"results":[{"index":0,"relevance_score":0.95},...]}`
type RerankClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type RerankConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func NewRerankClient(cfg RerankConfig) Reranker {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &RerankClient{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *RerankClient) Rerank(ctx context.Context, query string, docs []string) ([]float32, error) {
	reqBody := map[string]any{
		"model":     c.model,
		"query":     query,
		"documents": docs,
	}

	respBytes, err := doHTTPRequest(ctx, c.httpClient, c.baseURL+"/rerank", c.apiKey, reqBody)
	if err != nil {
		return nil, fmt.Errorf("rerank 请求失败: %w", err)
	}

	var result struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float32 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("解析 rerank 响应失败: %w", err)
	}

	scores := make([]float32, len(docs))
	for _, r := range result.Results {
		if r.Index >= 0 && r.Index < len(docs) {
			scores[r.Index] = r.RelevanceScore
		}
	}
	return scores, nil
}

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
