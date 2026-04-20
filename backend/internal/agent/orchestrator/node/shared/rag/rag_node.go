package rag

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"

	raginfra "github.com/XDWow/DouyinMall/backend/internal/agent/infra/rag"
)

type Input struct {
	Message string
	History []*schema.Message
	Intent  string
	Domains []string
}

type Result struct {
	Query     string
	Documents []*schema.Document
}

// RAGNode 知识库检索一步（rewrite/rerank 在库内）。
type RAGNode struct {
	KnowledgeBase raginfra.Searcher
	TopK          int
	MinScore      float64
}

func NewRAGNode(knowledgeBase raginfra.Searcher, topK int, minScore float64) *RAGNode {
	return &RAGNode{
		KnowledgeBase: knowledgeBase,
		TopK:          topK,
		MinScore:      minScore,
	}
}

func (n *RAGNode) Invoke(ctx context.Context, input Input) (*Result, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" || n.KnowledgeBase == nil {
		return &Result{}, nil
	}

	result, err := n.KnowledgeBase.Search(ctx, raginfra.SearchInput{
		Message:  message,
		History:  append([]*schema.Message(nil), input.History...),
		Intent:   strings.TrimSpace(input.Intent),
		TopK:     n.TopK,
		MinScore: n.MinScore,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &Result{}, nil
	}

	return &Result{
		Query:     strings.TrimSpace(result.Query),
		Documents: filterByDomains(result.Documents, input.Domains),
	}, nil
}

func filterByDomains(documents []*schema.Document, domains []string) []*schema.Document {
	if len(documents) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSpace(strings.ToLower(domain))
		if domain == "" {
			continue
		}
		allowed[domain] = struct{}{}
	}
	if len(allowed) == 0 {
		return append([]*schema.Document(nil), documents...)
	}

	filtered := make([]*schema.Document, 0, len(documents))
	for _, doc := range documents {
		if doc == nil {
			continue
		}
		if _, ok := allowed[documentDomain(doc)]; ok {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

func documentDomain(doc *schema.Document) string {
	if doc == nil || len(doc.MetaData) == 0 {
		return ""
	}
	if text, ok := doc.MetaData["domain"].(string); ok {
		return strings.TrimSpace(strings.ToLower(text))
	}
	if nested, ok := doc.MetaData["metadata"].(map[string]string); ok {
		return strings.TrimSpace(strings.ToLower(nested["domain"]))
	}
	if nested, ok := doc.MetaData["metadata"].(map[string]any); ok {
		if text, ok := nested["domain"].(string); ok {
			return strings.TrimSpace(strings.ToLower(text))
		}
	}
	return ""
}
