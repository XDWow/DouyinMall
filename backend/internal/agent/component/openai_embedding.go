package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

type OpenAIEmbeddingConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

type OpenAIEmbedder struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAIEmbedder(cfg OpenAIEmbeddingConfig) *OpenAIEmbedder {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &OpenAIEmbedder{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (e *OpenAIEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	options := embedding.GetCommonOptions(&embedding.Options{Model: &e.model}, opts...)
	body, err := json.Marshal(map[string]any{
		"model": valueOrDefault(options.Model, e.model),
		"input": texts,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(e.baseURL, "/embeddings"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding http %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	vectors := make([][]float64, 0, len(result.Data))
	for _, item := range result.Data {
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}

func (e *OpenAIEmbedder) GetType() string {
	return "OpenAICompatibleEmbedder"
}
