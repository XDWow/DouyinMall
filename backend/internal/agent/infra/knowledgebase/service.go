package knowledgebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

type KnowledgeServiceConfig struct {
	Scheme            string
	Domain            string
	ServiceChatPath   string
	ServiceResourceID string
	APIKey            string
	Timeout           time.Duration
}

// ManagedKnowledgeService 负责调用托管知识库服务，并把结果转换成 Eino 文档
// 托管侧内部会做 rewrite / embedding / 检索 / 重排，我们这里只负责请求与结果适配
type ManagedKnowledgeService struct {
	scheme            string
	domain            string
	serviceChatPath   string
	serviceResourceID string
	apiKey            string
	client            *http.Client
}

type SearchInput struct {
	Message  string
	History  []*schema.Message
	Intent   string
	TopK     int
	MinScore float64
}

type SearchResult struct {
	Query     string
	Documents []*schema.Document
}

type serviceChatRequest struct {
	ServiceResourceID string               `json:"service_resource_id,omitempty"`
	Stream            bool                 `json:"stream"`
	Messages          []serviceChatMessage `json:"messages"`
}

type serviceChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type serviceChatResponse struct {
	Code    int64                    `json:"code"`
	Message string                   `json:"message,omitempty"`
	Data    *serviceChatResponseData `json:"data,omitempty"`
}

type serviceChatResponseData struct {
	RewriteQuery string                    `json:"rewrite_query,omitempty"`
	ResultList   []*serviceSearchResultDoc `json:"result_list,omitempty"`
}

type serviceSearchResultDoc struct {
	ID             string                     `json:"id"`
	Content        string                     `json:"content"`
	MDContent      string                     `json:"md_content,omitempty"`
	Score          float64                    `json:"score"`
	PointID        string                     `json:"point_id"`
	OriginText     string                     `json:"origin_text,omitempty"`
	ChunkTitle     string                     `json:"chunk_title,omitempty"`
	RerankScore    float64                    `json:"rerank_score,omitempty"`
	RecallPosition int32                      `json:"recall_position"`
	RerankPosition int32                      `json:"rerank_position,omitempty"`
	ChunkType      string                     `json:"chunk_type,omitempty"`
	ChunkSource    string                     `json:"chunk_source,omitempty"`
	DocInfo        serviceSearchResultDocInfo `json:"doc_info,omitempty"`
}

type serviceSearchResultDocInfo struct {
	DocID   string `json:"doc_id"`
	DocName string `json:"doc_name"`
	DocType string `json:"doc_type"`
	Source  string `json:"source"`
	Title   string `json:"title,omitempty"`
}

func NewManagedKnowledgeService(cfg KnowledgeServiceConfig) (*ManagedKnowledgeService, error) {
	if strings.TrimSpace(cfg.Domain) == "" {
		return nil, fmt.Errorf("knowledge_base.domain is required")
	}
	if strings.TrimSpace(cfg.ServiceChatPath) == "" {
		return nil, fmt.Errorf("knowledge_base.service_chat_path is required")
	}
	if strings.TrimSpace(cfg.ServiceResourceID) == "" {
		return nil, fmt.Errorf("knowledge_base.service_resource_id is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("knowledge_base.api_key is required, please provide it through KNOWLEDGE_BASE_API_KEY")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	scheme := strings.TrimSpace(cfg.Scheme)
	if scheme == "" {
		scheme = "http"
	}

	return &ManagedKnowledgeService{
		scheme:            scheme,
		domain:            strings.TrimSpace(cfg.Domain),
		serviceChatPath:   strings.TrimSpace(cfg.ServiceChatPath),
		serviceResourceID: strings.TrimSpace(cfg.ServiceResourceID),
		apiKey:            strings.TrimSpace(cfg.APIKey),
		client:            &http.Client{Timeout: timeout},
	}, nil
}

// Search 把多轮消息发送到托管知识库，并返回排序后的文档结果。
func (s *ManagedKnowledgeService) Search(ctx context.Context, input SearchInput) (*SearchResult, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return &SearchResult{}, nil
	}

	body, err := json.Marshal(serviceChatRequest{
		ServiceResourceID: s.serviceResourceID,
		Stream:            false,
		Messages:          buildServiceMessages(input.History, message),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal knowledge service request failed: %w", err)
	}

	req, err := s.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request knowledge service failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read knowledge service response failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("knowledge service returned http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var payload serviceChatResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("decode knowledge service response failed: %w", err)
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("knowledge service returned code=%d message=%s", payload.Code, strings.TrimSpace(payload.Message))
	}

	result := &SearchResult{Query: message}
	if payload.Data == nil {
		return result, nil
	}
	if query := strings.TrimSpace(payload.Data.RewriteQuery); query != "" {
		result.Query = query
	}

	documents := make([]*schema.Document, 0, len(payload.Data.ResultList))
	for _, item := range payload.Data.ResultList {
		doc := toKnowledgeDocument(item, payload.Data.RewriteQuery)
		if doc == nil {
			continue
		}
		if input.MinScore > 0 && doc.Score() < input.MinScore {
			continue
		}
		documents = append(documents, doc)
	}
	if input.TopK > 0 && len(documents) > input.TopK {
		documents = append([]*schema.Document(nil), documents[:input.TopK]...)
	}
	result.Documents = documents
	return result, nil
}

func (s *ManagedKnowledgeService) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	u := url.URL{
		Scheme: s.scheme,
		Host:   s.domain,
		Path:   s.serviceChatPath,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build knowledge service request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", s.domain)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	return req, nil
}

func buildServiceMessages(history []*schema.Message, message string) []serviceChatMessage {
	messages := make([]serviceChatMessage, 0, len(history)+1)
	for _, item := range history {
		if item == nil {
			continue
		}
		role := normalizeServiceRole(string(item.Role))
		content := strings.TrimSpace(item.Content)
		if role == "" || content == "" {
			continue
		}
		messages = append(messages, serviceChatMessage{
			Role:    role,
			Content: content,
		})
	}
	messages = append(messages, serviceChatMessage{
		Role:    "user",
		Content: message,
	})
	return messages
}

func normalizeServiceRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "assistant"
	case "user":
		return "user"
	default:
		return ""
	}
}

func toKnowledgeDocument(item *serviceSearchResultDoc, rewriteQuery string) *schema.Document {
	if item == nil {
		return nil
	}
	content := firstNonEmpty(item.MDContent, item.Content, item.OriginText)
	if strings.TrimSpace(content) == "" {
		return nil
	}

	doc := &schema.Document{
		ID:      firstNonEmpty(item.ID, item.PointID, item.DocInfo.DocID),
		Content: content,
		MetaData: map[string]any{
			"title":                     firstNonEmpty(item.ChunkTitle, item.DocInfo.Title, item.DocInfo.DocName),
			"category":                  firstNonEmpty(item.ChunkSource, item.ChunkType, item.DocInfo.DocType, "知识库"),
			"snippet":                   firstNonEmpty(item.OriginText, item.Content, item.MDContent),
			"doc_id":                    strings.TrimSpace(item.DocInfo.DocID),
			"doc_name":                  strings.TrimSpace(item.DocInfo.DocName),
			"doc_type":                  strings.TrimSpace(item.DocInfo.DocType),
			"source":                    strings.TrimSpace(item.DocInfo.Source),
			"knowledge_service":         "volcengine",
			"knowledge_rewrite_query":   strings.TrimSpace(rewriteQuery),
			"knowledge_recall_position": int(item.RecallPosition),
			"knowledge_rerank_position": int(item.RerankPosition),
			"knowledge_rerank_score":    chooseScore(item.RerankScore, item.Score),
		},
	}
	return doc.WithScore(chooseScore(item.RerankScore, item.Score))
}

func chooseScore(primary, fallback float64) float64 {
	if primary > 0 {
		return primary
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
