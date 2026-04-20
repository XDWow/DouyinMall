package rag

import (
	"context"
	"fmt"
	"strings"

	qdrantretriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

type LocalQdrantConfig struct {
	Host            string
	Port            int
	APIKey          string
	Collection      string
	UseTLS          bool
	DefaultTopK     int
	DefaultMinScore float64
	Embedder        embedding.Embedder
}

type LocalQdrantKnowledgeService struct {
	retriever       *qdrantretriever.Retriever
	defaultTopK     int
	defaultMinScore float64
}

func NewLocalQdrantKnowledgeService(ctx context.Context, cfg LocalQdrantConfig) (*LocalQdrantKnowledgeService, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("local qdrant embedder is required")
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("local qdrant host is required")
	}
	if strings.TrimSpace(cfg.Collection) == "" {
		return nil, fmt.Errorf("local qdrant collection is required")
	}

	port := cfg.Port
	if port <= 0 {
		port = 6334
	}
	topK := cfg.DefaultTopK
	if topK <= 0 {
		topK = 5
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   strings.TrimSpace(cfg.Host),
		Port:   port,
		APIKey: strings.TrimSpace(cfg.APIKey),
		UseTLS: cfg.UseTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("init qdrant client failed: %w", err)
	}

	retrieverConfig := &qdrantretriever.Config{
		Client:     client,
		Collection: strings.TrimSpace(cfg.Collection),
		Embedding:  cfg.Embedder,
		TopK:       topK,
	}
	if cfg.DefaultMinScore > 0 {
		retrieverConfig.ScoreThreshold = &cfg.DefaultMinScore
	}

	retriever, err := qdrantretriever.NewRetriever(ctx, retrieverConfig)
	if err != nil {
		return nil, fmt.Errorf("init qdrant retriever failed: %w", err)
	}

	return &LocalQdrantKnowledgeService{
		retriever:       retriever,
		defaultTopK:     topK,
		defaultMinScore: cfg.DefaultMinScore,
	}, nil
}

func (s *LocalQdrantKnowledgeService) Search(ctx context.Context, input SearchInput) (*SearchResult, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return &SearchResult{}, nil
	}

	topK := input.TopK
	if topK <= 0 {
		topK = s.defaultTopK
	}
	minScore := input.MinScore
	if minScore <= 0 {
		minScore = s.defaultMinScore
	}

	opts := make([]einoretriever.Option, 0, 1)
	if topK > 0 {
		opts = append(opts, einoretriever.WithTopK(topK))
	}

	docs, err := s.retriever.Retrieve(ctx, message, opts...)
	if err != nil {
		return nil, fmt.Errorf("qdrant retrieve failed: %w", err)
	}

	return &SearchResult{
		Query:     message,
		Documents: normalizeQdrantRetrievedDocuments(docs, minScore),
	}, nil
}

func normalizeQdrantRetrievedDocuments(docs []*schema.Document, minScore float64) []*schema.Document {
	if len(docs) == 0 {
		return nil
	}

	normalized := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		item := normalizeQdrantRetrievedDocument(doc)
		if item == nil {
			continue
		}
		if minScore > 0 && item.Score() < minScore {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func normalizeQdrantRetrievedDocument(doc *schema.Document) *schema.Document {
	if doc == nil || strings.TrimSpace(doc.Content) == "" {
		return nil
	}

	meta := unwrapQdrantMetadata(doc.MetaData)
	if meta == nil {
		meta = map[string]any{}
	}
	if snippet := firstNonEmpty(stringifyAny(meta["snippet"]), summarizeSnippet(doc.Content)); snippet != "" {
		meta["snippet"] = snippet
	}

	normalized := &schema.Document{
		ID:       strings.TrimSpace(doc.ID),
		Content:  strings.TrimSpace(doc.Content),
		MetaData: meta,
	}
	return normalized.WithScore(doc.Score())
}

func unwrapQdrantMetadata(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}

	meta := make(map[string]any, len(raw))
	for key, value := range raw {
		if key == "metadata" {
			continue
		}
		meta[key] = value
	}

	if nested := qdrantMetadataToAny(raw["metadata"]); len(nested) > 0 {
		for key, value := range nested {
			meta[key] = value
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func qdrantMetadataToAny(raw any) map[string]any {
	switch typed := raw.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, value := range typed {
			cloned[key] = value
		}
		return cloned
	case map[string]*qdrant.Value:
		return qdrantValueMapToAny(typed)
	case *qdrant.Struct:
		return qdrantValueMapToAny(typed.GetFields())
	default:
		return nil
	}
}

func qdrantValueMapToAny(fields map[string]*qdrant.Value) map[string]any {
	if len(fields) == 0 {
		return nil
	}

	converted := make(map[string]any, len(fields))
	for key, value := range fields {
		converted[key] = qdrantValueToAny(value)
	}
	return converted
}

func qdrantValueToAny(value *qdrant.Value) any {
	if value == nil {
		return nil
	}

	switch typed := value.GetKind().(type) {
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_DoubleValue:
		return typed.DoubleValue
	case *qdrant.Value_IntegerValue:
		return typed.IntegerValue
	case *qdrant.Value_StringValue:
		return typed.StringValue
	case *qdrant.Value_BoolValue:
		return typed.BoolValue
	case *qdrant.Value_StructValue:
		return qdrantValueMapToAny(typed.StructValue.GetFields())
	case *qdrant.Value_ListValue:
		values := typed.ListValue.GetValues()
		items := make([]any, 0, len(values))
		for _, item := range values {
			items = append(items, qdrantValueToAny(item))
		}
		return items
	default:
		return nil
	}
}

func summarizeSnippet(content string) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= 140 {
		return content
	}
	return string(runes[:140]) + "..."
}

func stringifyAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

var _ Searcher = (*LocalQdrantKnowledgeService)(nil)
