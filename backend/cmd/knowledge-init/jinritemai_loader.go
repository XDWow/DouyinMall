package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

type jinritemaiArticleLoader struct {
	client  *http.Client
	graphID int
	baseURL string
}

type jinritemaiArticleDetailResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Data    struct {
		ArticleInfo jinritemaiArticleInfo `json:"article_info"`
	} `json:"data"`
}

type jinritemaiArticleInfo struct {
	ArticleID       string   `json:"article_id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Content         string   `json:"content"`
	CreatorName     string   `json:"creator_name"`
	UpdateTimestamp int64    `json:"update_timestamp"`
	Tags            []string `json:"tags"`
	OfflineStatus   int      `json:"offline_status"`
	ShowComment     bool     `json:"show_comment"`
}

type jinritemaiRichText struct {
	Deltas map[string]jinritemaiDelta `json:"deltas"`
}

type jinritemaiDelta struct {
	Ops []jinritemaiOp `json:"ops"`
}

type jinritemaiOp struct {
	Insert     any            `json:"insert"`
	Attributes map[string]any `json:"attributes"`
}

func newJinritemaiArticleLoader(client *http.Client, graphID int) document.Loader {
	return &jinritemaiArticleLoader{
		client:  client,
		graphID: graphID,
		baseURL: "https://school.jinritemai.com",
	}
}

func (l *jinritemaiArticleLoader) Load(ctx context.Context, src document.Source, _ ...document.LoaderOption) ([]*schema.Document, error) {
	articleID, err := extractJinritemaiArticleID(src.URI)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"%s/api/eschool/v2/library/article/detail?id=%s&graphId=%d&need_content=true",
		strings.TrimRight(l.baseURL, "/"),
		neturl.QueryEscape(articleID),
		l.graphID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build jinritemai article request failed: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request jinritemai article failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read jinritemai article response failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("jinritemai article returned http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload jinritemaiArticleDetailResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode jinritemai article response failed: %w", err)
	}
	if payload.Code != 0 {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = strings.TrimSpace(payload.Msg)
		}
		return nil, fmt.Errorf("jinritemai article returned code=%d message=%s", payload.Code, message)
	}

	info := payload.Data.ArticleInfo
	content := extractPlainTextFromJinritemaiContent(info.Content)
	if content == "" {
		content = strings.TrimSpace(info.Description)
	}
	if content == "" {
		return nil, fmt.Errorf("jinritemai article %s returned empty content", articleID)
	}

	title := strings.TrimSpace(info.Name)
	if title == "" {
		title = "Jinritemai Article " + articleID
	}

	return []*schema.Document{
		{
			ID:      info.ArticleID,
			Content: content,
			MetaData: map[string]any{
				"title":            title,
				"description":      strings.TrimSpace(info.Description),
				"source_url":       src.URI,
				"article_id":       info.ArticleID,
				"creator_name":     strings.TrimSpace(info.CreatorName),
				"update_timestamp": info.UpdateTimestamp,
				"tags":             info.Tags,
				"loader":           "jinritemai_article",
			},
		},
	}, nil
}

func isJinritemaiArticleURL(raw string) bool {
	_, err := extractJinritemaiArticleID(raw)
	return err == nil
}

func extractJinritemaiArticleID(raw string) (string, error) {
	parsed, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse source URL failed: %w", err)
	}
	if !strings.EqualFold(parsed.Hostname(), "school.jinritemai.com") {
		return "", fmt.Errorf("unsupported host for jinritemai loader: %s", parsed.Hostname())
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "doudian" || parts[1] != "web" || parts[2] != "article" {
		return "", fmt.Errorf("unsupported jinritemai article path: %s", parsed.Path)
	}
	if strings.TrimSpace(parts[3]) == "" {
		return "", fmt.Errorf("jinritemai article id is missing in %s", raw)
	}
	return parts[3], nil
}

func extractPlainTextFromJinritemaiContent(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var richText jinritemaiRichText
	if err := json.Unmarshal([]byte(raw), &richText); err != nil {
		return normalizeExtractedText(raw)
	}

	keys := make([]int, 0, len(richText.Deltas))
	keyMap := make(map[int]string, len(richText.Deltas))
	for key := range richText.Deltas {
		index, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		keys = append(keys, index)
		keyMap[index] = key
	}
	sort.Ints(keys)

	var builder strings.Builder
	for _, index := range keys {
		delta := richText.Deltas[keyMap[index]]
		appendJinritemaiOps(&builder, delta.Ops)
	}
	return normalizeExtractedText(builder.String())
}

func appendJinritemaiOps(builder *strings.Builder, ops []jinritemaiOp) {
	for _, op := range ops {
		switch inserted := op.Insert.(type) {
		case string:
			if isJinritemaiMarker(inserted, op.Attributes) {
				writeJinritemaiPrefix(builder, op.Attributes)
				continue
			}
			if isHorizontalRule(op.Attributes) {
				ensureTrailingNewline(builder)
				continue
			}
			builder.WriteString(inserted)
		default:
			// Ignore non-text inserts such as images or embedded widgets.
		}
	}
}

func isJinritemaiMarker(inserted string, attrs map[string]any) bool {
	return inserted == "*" && attrs != nil && attrs["lmkr"] != nil
}

func isHorizontalRule(attrs map[string]any) bool {
	return attrs != nil && attrs["horizontal-line"] != nil
}

func writeJinritemaiPrefix(builder *strings.Builder, attrs map[string]any) {
	if attrs == nil {
		return
	}
	if attrs["list"] != nil {
		ensureTrailingNewline(builder)
		builder.WriteString("- ")
		return
	}
	if attrs["heading"] != nil {
		ensureTrailingNewline(builder)
	}
}

func ensureTrailingNewline(builder *strings.Builder) {
	current := builder.String()
	if current == "" || strings.HasSuffix(current, "\n") {
		return
	}
	builder.WriteByte('\n')
}
