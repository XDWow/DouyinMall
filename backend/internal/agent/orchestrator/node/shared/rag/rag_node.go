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
}

type Result struct {
	Query     string
	Documents []*schema.Document
}

// RAGNode 知识库检索一步（rewrite/rerank 在库内）。
type RAGNode struct {
	KnowledgeBase *raginfra.ManagedKnowledgeService
	TopK          int
	MinScore      float64
}

func NewRAGNode(knowledgeBase *raginfra.ManagedKnowledgeService, topK int, minScore float64) *RAGNode {
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
		Documents: append([]*schema.Document(nil), result.Documents...),
	}, nil
}
