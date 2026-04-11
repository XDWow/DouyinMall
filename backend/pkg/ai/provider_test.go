package ai

import "testing"

func TestResolveOpenAIBaseURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider Provider
		rawURL   string
		want     string
	}{
		{
			name:     "iFlow 根地址自动补 v1",
			provider: ProviderIFlow,
			rawURL:   "https://apis.iflow.cn",
			want:     "https://apis.iflow.cn/v1",
		},
		{
			name:     "SiliconFlow 保留已有 v1",
			provider: ProviderSiliconFlow,
			rawURL:   "https://api.siliconflow.cn/v1",
			want:     "https://api.siliconflow.cn/v1",
		},
		{
			name:     "Ark 自动补 api v3",
			provider: ProviderVolcengineArk,
			rawURL:   "https://ark.cn-beijing.volces.com",
			want:     "https://ark.cn-beijing.volces.com/api/v3",
		},
		{
			name:     "从完整聊天接口回退到根地址",
			provider: ProviderIFlow,
			rawURL:   "https://apis.iflow.cn/v1/chat/completions",
			want:     "https://apis.iflow.cn/v1",
		},
		{
			name:     "从完整重排接口回退到根地址",
			provider: ProviderSiliconFlow,
			rawURL:   "https://api.siliconflow.cn/v1/rerank",
			want:     "https://api.siliconflow.cn/v1",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolveOpenAIBaseURL(tc.provider, tc.rawURL); got != tc.want {
				t.Fatalf("期望 %s，实际 %s", tc.want, got)
			}
		})
	}
}

func TestResolveCapabilityEndpoint(t *testing.T) {
	t.Parallel()

	if got := ResolveChatEndpoint(ProviderIFlow, "https://apis.iflow.cn"); got != "https://apis.iflow.cn/v1/chat/completions" {
		t.Fatalf("聊天接口解析错误: %s", got)
	}
	if got := ResolveEmbeddingEndpoint(ProviderOllama, "http://127.0.0.1:11434"); got != "http://127.0.0.1:11434/v1/embeddings" {
		t.Fatalf("向量接口解析错误: %s", got)
	}
	if got := ResolveMultimodalEmbeddingEndpoint(ProviderVolcengineArk, "https://ark.cn-beijing.volces.com"); got != "https://ark.cn-beijing.volces.com/api/v3/embeddings/multimodal" {
		t.Fatalf("多模态向量接口解析错误: %s", got)
	}
	if got := ResolveRerankEndpoint(ProviderSiliconFlow, "https://api.siliconflow.cn"); got != "https://api.siliconflow.cn/v1/rerank" {
		t.Fatalf("重排接口解析错误: %s", got)
	}
}
