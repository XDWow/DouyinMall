package main

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	urlloader "github.com/cloudwego/eino-ext/components/document/loader/url"
	htmlparser "github.com/cloudwego/eino-ext/components/document/parser/html"
	recursive "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

const defaultUserAgent = "Mozilla/5.0 (compatible; DouyinMallKnowledgeInit/1.0)"

var nonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

type PrepareRequest struct {
	URL         string
	KnowledgeID string
	Title       string
	Category    string
}

type StoreRequest struct {
	Artifact *KnowledgeArtifact
	Replace  bool
}

type IngestResult struct {
	KnowledgeID      string
	Title            string
	Category         string
	RawDocumentCount int
	ChunkCount       int
	SourceURL        string
}

type prepareService struct {
	genericLoader    document.Loader
	jinritemaiLoader document.Loader
	splitter         document.Transformer
	chunkSize        int
	defaultCategory  string
}

type storeService struct {
	embedder     embedding.Embedder
	qdrantClient *qdrant.Client
	qdrantConfig QdrantConfig
}

func newPrepareService(ctx context.Context, cfg Config, selectorOverride string) (*prepareService, func(), error) {
	httpClient := &http.Client{Timeout: time.Duration(cfg.Ingest.HTTPTimeoutSeconds) * time.Second}

	genericLoader, err := newGenericURLLoader(ctx, httpClient, chooseSelector(cfg.Ingest.Selector, selectorOverride))
	if err != nil {
		return nil, nil, fmt.Errorf("init generic URL loader failed: %w", err)
	}

	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   cfg.Ingest.ChunkSize,
		OverlapSize: cfg.Ingest.OverlapSize,
		Separators:  []string{"\n\n", "\n", "。", "；", "！", "？", ".", ";", "!", "?"},
		KeepType:    recursive.KeepTypeEnd,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("init recursive splitter failed: %w", err)
	}

	return &prepareService{
		genericLoader:    genericLoader,
		jinritemaiLoader: newJinritemaiArticleLoader(httpClient, cfg.Ingest.JinritemaiGraphID),
		splitter:         splitter,
		chunkSize:        cfg.Ingest.ChunkSize,
		defaultCategory:  cfg.Ingest.DefaultCategory,
	}, func() {}, nil
}

func newStoreService(ctx context.Context, cfg Config) (*storeService, func(), error) {
	embedder, err := pkgai.NewEinoEmbedder(ctx, pkgai.EinoEmbeddingConfig{
		Provider: cfg.Embedding.Provider,
		BaseURL:  cfg.Embedding.BaseURL,
		APIKey:   cfg.Embedding.APIKey,
		Model:    cfg.Embedding.Model,
		Timeout:  time.Duration(cfg.Embedding.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("init embedder failed: %w", err)
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   strings.TrimSpace(cfg.Qdrant.Host),
		Port:   cfg.Qdrant.Port,
		APIKey: strings.TrimSpace(cfg.Qdrant.APIKey),
		UseTLS: cfg.Qdrant.UseTLS,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("init qdrant client failed: %w", err)
	}

	return &storeService{
		embedder:     embedder,
		qdrantClient: client,
		qdrantConfig: cfg.Qdrant,
	}, func() {}, nil
}

func (s *prepareService) PrepareURL(ctx context.Context, req PrepareRequest) (*KnowledgeArtifact, error) {
	sourceURL := strings.TrimSpace(req.URL)
	if sourceURL == "" {
		return nil, fmt.Errorf("source URL is required")
	}

	loader := s.genericLoader
	if isJinritemaiArticleURL(sourceURL) {
		loader = s.jinritemaiLoader
	}

	docs, err := loader.Load(ctx, document.Source{URI: sourceURL})
	if err != nil {
		return nil, fmt.Errorf("load source failed: %w", err)
	}
	docs = normalizeDocuments(docs)
	if len(docs) == 0 {
		return nil, fmt.Errorf("no usable content extracted from %s", sourceURL)
	}

	loadedTitle := strings.TrimSpace(req.Title)
	if loadedTitle == "" {
		loadedTitle = titleFromDocuments(docs)
	}
	if loadedTitle == "" {
		loadedTitle = "Untitled Knowledge"
	}

	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = s.defaultCategory
	}

	knowledgeID := strings.TrimSpace(req.KnowledgeID)
	if knowledgeID == "" {
		knowledgeID = deriveKnowledgeID(sourceURL, docs)
	}
	if knowledgeID == "" {
		return nil, fmt.Errorf("failed to derive knowledge id from %s", sourceURL)
	}

	chunks, err := s.splitDocuments(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("split content failed: %w", err)
	}
	chunks = normalizeDocuments(chunks)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("splitter returned no content chunks")
	}

	artifact := &KnowledgeArtifact{
		Version:     "v1",
		GeneratedAt: time.Now().UTC(),
		SourceURL:   sourceURL,
		KnowledgeID: knowledgeID,
		Title:       loadedTitle,
		Category:    category,
		Documents:   make([]ArtifactDocument, 0, len(docs)),
		Chunks:      make([]ArtifactChunk, 0, len(chunks)),
	}

	for _, doc := range docs {
		artifact.Documents = append(artifact.Documents, ArtifactDocument{
			ID:       doc.ID,
			Content:  doc.Content,
			Metadata: stringifyMetadataMap(doc.MetaData),
		})
	}

	for idx, chunk := range chunks {
		chunkID := fmt.Sprintf("%s-%03d", knowledgeID, idx+1)
		artifact.Chunks = append(artifact.Chunks, ArtifactChunk{
			ID:       chunkID,
			Content:  chunk.Content,
			Snippet:  summarizeSnippet(chunk.Content),
			Metadata: buildChunkMetadata(sourceURL, chunk, idx, len(chunks)),
		})
	}

	return artifact, nil
}

func (s *storeService) StoreArtifact(ctx context.Context, req StoreRequest) (*IngestResult, error) {
	artifact := req.Artifact
	if artifact == nil {
		return nil, fmt.Errorf("artifact is required")
	}
	if len(artifact.Chunks) == 0 {
		return nil, fmt.Errorf("artifact has no chunks to store")
	}
	if err := storeArtifactToQdrant(ctx, s.qdrantClient, s.embedder, s.qdrantConfig, req); err != nil {
		return nil, err
	}

	return &IngestResult{
		KnowledgeID:      artifact.KnowledgeID,
		Title:            artifact.Title,
		Category:         artifact.Category,
		RawDocumentCount: len(artifact.Documents),
		ChunkCount:       len(artifact.Chunks),
		SourceURL:        artifact.SourceURL,
	}, nil
}

func newGenericURLLoader(ctx context.Context, client *http.Client, selector string) (document.Loader, error) {
	var parserSelector *string
	if strings.TrimSpace(selector) != "" {
		parserSelector = &selector
	}

	parser, err := htmlparser.NewParser(ctx, &htmlparser.Config{
		Selector: parserSelector,
	})
	if err != nil {
		return nil, err
	}

	return urlloader.NewLoader(ctx, &urlloader.LoaderConfig{
		Parser: parser,
		Client: client,
		RequestBuilder: func(ctx context.Context, source document.Source, _ ...document.LoaderOption) (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URI, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("User-Agent", defaultUserAgent)
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
			return req, nil
		},
	})
}

func normalizeDocuments(docs []*schema.Document) []*schema.Document {
	normalized := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		doc.Content = normalizeExtractedText(doc.Content)
		if strings.TrimSpace(doc.Content) == "" {
			continue
		}
		normalized = append(normalized, doc)
	}
	return normalized
}

func chooseSelector(configSelector, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	return strings.TrimSpace(configSelector)
}

func titleFromDocuments(docs []*schema.Document) string {
	for _, doc := range docs {
		if title := titleFromDocument(doc); title != "" {
			return title
		}
	}
	return ""
}

func titleFromDocument(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	if doc.MetaData != nil {
		for _, key := range []string{"title", "_title", "name"} {
			if title := stringifyMetaValue(doc.MetaData[key]); title != "" {
				return title
			}
		}
	}
	for _, line := range strings.Split(doc.Content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 80)
		}
	}
	return ""
}

func buildChunkMetadata(sourceURL string, doc *schema.Document, chunkIndex, total int) map[string]string {
	meta := map[string]string{
		"source_url":  sourceURL,
		"chunk_index": strconv.Itoa(chunkIndex + 1),
		"chunk_total": strconv.Itoa(total),
	}
	if doc == nil || doc.MetaData == nil {
		return meta
	}

	for _, key := range []string{
		"title", "_title", "article_id", "description", "loader", "creator_name",
		"title_path", "heading_title", "heading_level", "section_number",
		"chapter_title", "section_title", "clause_title", "subclause_title",
		"heading_kind", "heading_number_path", "chunk_strategy",
	} {
		if value := stringifyMetaValue(doc.MetaData[key]); value != "" {
			meta[key] = value
		}
	}
	if tags := stringifyStringSlice(doc.MetaData["tags"]); tags != "" {
		meta["tags"] = tags
	}
	if value := stringifyMetaValue(doc.MetaData["update_timestamp"]); value != "" {
		meta["update_timestamp"] = value
	}
	return meta
}

func deriveKnowledgeID(sourceURL string, docs []*schema.Document) string {
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if articleID := stringifyMetaValue(doc.MetaData["article_id"]); articleID != "" {
			return "jinritemai_article_" + sanitizeSlug(articleID)
		}
	}

	parsed, err := neturl.Parse(sourceURL)
	if err != nil {
		return sanitizeSlug(sourceURL)
	}

	host := sanitizeSlug(parsed.Hostname())
	path := sanitizeSlug(strings.Trim(parsed.Path, "/"))
	switch {
	case host != "" && path != "":
		return host + "_" + path
	case host != "":
		return host
	default:
		return sanitizeSlug(sourceURL)
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

func sanitizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonAlphaNumeric.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "knowledge"
	}
	return value
}

func stringifyMetaValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func stringifyStringSlice(value any) string {
	switch typed := value.(type) {
	case []string:
		return strings.Join(typed, ",")
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringifyMetaValue(item); text != "" {
				items = append(items, text)
			}
		}
		return strings.Join(items, ",")
	default:
		return stringifyMetaValue(value)
	}
}

func stringifyMetadataMap(raw map[string]any) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	meta := make(map[string]string, len(raw))
	for key, value := range raw {
		if text := stringifyStringSlice(value); text != "" {
			meta[key] = text
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func cloneStringMap(raw map[string]string) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(raw))
	for key, value := range raw {
		cloned[key] = value
	}
	return cloned
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func normalizeExtractedText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\u00a0", " ")

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !lastBlank && len(cleaned) > 0 {
				cleaned = append(cleaned, "")
			}
			lastBlank = true
			continue
		}
		if line == "*" || line == "-" || line == "\u2022" {
			lastBlank = false
			continue
		}
		cleaned = append(cleaned, line)
		lastBlank = false
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
