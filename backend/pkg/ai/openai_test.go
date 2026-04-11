package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatClientUsesDefaultModel(t *testing.T) {
	t.Parallel()

	var got struct {
		Model string `json:"model"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("请求路径错误: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(ChatConfig{
		Provider: ProviderIFlow,
		BaseURL:  server.URL,
		Model:    "qwen3-max",
	})

	resp, err := client.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "你好"}},
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if got.Model != "qwen3-max" {
		t.Fatalf("默认模型未写入请求，实际为 %s", got.Model)
	}
	if resp == nil || len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("响应解析失败: %+v", resp)
	}
}

func TestEmbeddingClientParsesOpenAIResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("请求路径错误: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]},{"index":1,"embedding":[0.3,0.4]}]}`))
	}))
	defer server.Close()

	client := NewEmbeddingClient(EmbeddingConfig{
		Provider: ProviderSiliconFlow,
		BaseURL:  server.URL,
		Model:    "bge-large-zh",
	})

	vectors, err := client.Embed(context.Background(), []string{"A", "B"})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("向量数量错误: %d", len(vectors))
	}
	if len(vectors[0]) != 2 || vectors[0][0] != 0.1 || vectors[1][1] != 0.4 {
		t.Fatalf("向量内容错误: %+v", vectors)
	}
}

func TestMultimodalEmbeddingUsesProviderPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/embeddings/multimodal" {
			t.Fatalf("请求路径错误: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2,3]}]}`))
	}))
	defer server.Close()

	client := NewEmbeddingClient(EmbeddingConfig{
		Provider: ProviderVolcengineArk,
		BaseURL:  server.URL,
		Model:    "ep-xxx",
	})

	vectors, err := client.EmbedMultimodal(context.Background(), []EmbeddingInput{
		{Type: "text", Text: "天很蓝"},
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if len(vectors) != 1 || len(vectors[0]) != 3 {
		t.Fatalf("向量内容错误: %+v", vectors)
	}
}
