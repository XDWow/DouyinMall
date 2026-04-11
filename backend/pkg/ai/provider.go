package ai

import (
	"net/url"
	"strings"
)

type Provider string

const (
	ProviderOpenAICompatible Provider = "openai_compatible"
	ProviderIFlow            Provider = "iflow"
	ProviderSiliconFlow      Provider = "siliconflow"
	ProviderOllama           Provider = "ollama"
	ProviderVolcengineArk    Provider = "volcengine_ark"
)

type Capability string

const (
	CapabilityChat                Capability = "chat"
	CapabilityEmbedding           Capability = "embedding"
	CapabilityMultimodalEmbedding Capability = "multimodal_embedding"
	CapabilityRerank              Capability = "rerank"
)

type providerProfile struct {
	defaultBasePath         string
	chatPath                string
	embeddingPath           string
	multimodalEmbeddingPath string
	rerankPath              string
}

func NormalizeProvider(raw string) Provider {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "openai", "openai_compatible", "openai-compatible", "compatible":
		return ProviderOpenAICompatible
	case "iflow":
		return ProviderIFlow
	case "siliconflow", "silicon_flow":
		return ProviderSiliconFlow
	case "ollama":
		return ProviderOllama
	case "volcengine_ark", "volcengine-ark", "volces_ark", "volces-ark", "ark", "doubao":
		return ProviderVolcengineArk
	default:
		return Provider(strings.ToLower(strings.TrimSpace(raw)))
	}
}

func ResolveOpenAIBaseURL(provider Provider, rawBaseURL string) string {
	profile := profileForProvider(provider)
	return normalizeBaseURL(profile, rawBaseURL)
}

func ResolveChatEndpoint(provider Provider, rawBaseURL string) string {
	profile := profileForProvider(provider)
	return joinURL(normalizeBaseURL(profile, rawBaseURL), profile.chatPath)
}

func ResolveEmbeddingEndpoint(provider Provider, rawBaseURL string) string {
	profile := profileForProvider(provider)
	return joinURL(normalizeBaseURL(profile, rawBaseURL), profile.embeddingPath)
}

func ResolveMultimodalEmbeddingEndpoint(provider Provider, rawBaseURL string) string {
	profile := profileForProvider(provider)
	return joinURL(normalizeBaseURL(profile, rawBaseURL), profile.multimodalEmbeddingPath)
}

func ResolveRerankEndpoint(provider Provider, rawBaseURL string) string {
	profile := profileForProvider(provider)
	return joinURL(normalizeBaseURL(profile, rawBaseURL), profile.rerankPath)
}

func profileForProvider(provider Provider) providerProfile {
	switch NormalizeProvider(string(provider)) {
	case ProviderIFlow, ProviderSiliconFlow, ProviderOllama, ProviderOpenAICompatible:
		return providerProfile{
			defaultBasePath:         "/v1",
			chatPath:                "/chat/completions",
			embeddingPath:           "/embeddings",
			multimodalEmbeddingPath: "/embeddings/multimodal",
			rerankPath:              "/rerank",
		}
	case ProviderVolcengineArk:
		return providerProfile{
			defaultBasePath:         "/api/v3",
			chatPath:                "/chat/completions",
			embeddingPath:           "/embeddings",
			multimodalEmbeddingPath: "/embeddings/multimodal",
			rerankPath:              "/rerank",
		}
	default:
		return providerProfile{
			chatPath:                "/chat/completions",
			embeddingPath:           "/embeddings",
			multimodalEmbeddingPath: "/embeddings/multimodal",
			rerankPath:              "/rerank",
		}
	}
}

func normalizeBaseURL(profile providerProfile, rawBaseURL string) string {
	rawBaseURL = strings.TrimSpace(rawBaseURL)
	if rawBaseURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawBaseURL)
	if err != nil {
		return strings.TrimRight(rawBaseURL, "/")
	}

	path := stripKnownCapabilityPath(parsed.Path)
	if path == "" || path == "/" {
		path = profile.defaultBasePath
	}
	parsed.Path = strings.TrimRight(path, "/")

	return strings.TrimRight(parsed.String(), "/")
}

func stripKnownCapabilityPath(path string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return ""
	}

	suffixes := []string{
		"/chat/completions",
		"/embeddings/multimodal",
		"/embeddings",
		"/rerank",
		"/api/embed",
		"/api/embeddings",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(trimmed, suffix) {
			return strings.TrimSuffix(trimmed, suffix)
		}
	}
	return trimmed
}

func joinURL(baseURL, suffix string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	suffix = "/" + strings.TrimLeft(strings.TrimSpace(suffix), "/")

	if baseURL == "" {
		return suffix
	}
	return baseURL + suffix
}
