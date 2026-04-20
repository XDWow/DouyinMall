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
	provider   Provider
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type ChatConfig struct {
	Provider Provider
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
}

func NewOpenAIClient(cfg ChatConfig) LLMClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &llmClient{
		provider:   NormalizeProvider(string(cfg.Provider)),
		baseURL:    ResolveOpenAIBaseURL(cfg.Provider, cfg.BaseURL),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *llmClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req = c.buildRequestBody(req)

	respBody, err := doHTTPRequest(ctx, c.httpClient, ResolveChatEndpoint(c.provider, c.baseURL), c.apiKey, req)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	var result ChatResponse
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode chat response failed: %w", err)
	}
	if len(result.Choices) == 0 {
		err = errors.New("empty chat choices")
	}

	return &result, err
}

func (c *llmClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamResponse, error) {
	req = c.buildRequestBody(req)

	bodyJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ResolveChatEndpoint(c.provider, c.baseURL), bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
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

func (c *llmClient) buildRequestBody(req ChatRequest) ChatRequest {
	if strings.TrimSpace(req.Model) == "" {
		req.Model = c.model
	}
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

type EmbeddingClient struct {
	provider   Provider
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type EmbeddingConfig struct {
	Provider Provider
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
}

func NewEmbeddingClient(cfg EmbeddingConfig) *EmbeddingClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &EmbeddingClient{
		provider:   NormalizeProvider(string(cfg.Provider)),
		baseURL:    ResolveOpenAIBaseURL(cfg.Provider, cfg.BaseURL),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body := map[string]any{
		"model": c.model,
		"input": texts,
	}
	respBody, err := doHTTPRequest(ctx, c.httpClient, ResolveEmbeddingEndpoint(c.provider, c.baseURL), c.apiKey, body)
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}
	return parseEmbeddingResponse(respBody)
}

func (c *EmbeddingClient) EmbedMultimodal(ctx context.Context, inputs []EmbeddingInput) ([][]float32, error) {
	body := map[string]any{
		"model": c.model,
		"input": inputs,
	}
	respBody, err := doHTTPRequest(ctx, c.httpClient, ResolveMultimodalEmbeddingEndpoint(c.provider, c.baseURL), c.apiKey, body)
	if err != nil {
		return nil, fmt.Errorf("multimodal embedding failed: %w", err)
	}
	return parseEmbeddingResponse(respBody)
}

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
		return nil, fmt.Errorf("rerank request failed: %w", err)
	}

	var result struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float32 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("decode rerank response failed: %w", err)
	}

	scores := make([]float32, len(docs))
	for _, r := range result.Results {
		if r.Index >= 0 && r.Index < len(docs) {
			scores[r.Index] = r.RelevanceScore
		}
	}
	return scores, nil
}

func doHTTPRequest(ctx context.Context, client *http.Client, url, apiKey string, body any) ([]byte, error) {
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

func parseEmbeddingResponse(respBody []byte) ([][]float32, error) {
	var openAIResp struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &openAIResp); err == nil && len(openAIResp.Data) > 0 {
		vectors := make([][]float32, len(openAIResp.Data))
		for i, item := range openAIResp.Data {
			vectors[i] = item.Embedding
		}
		return vectors, nil
	}

	var legacyResp struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(respBody, &legacyResp); err != nil {
		return nil, err
	}
	return legacyResp.Embeddings, nil
}
