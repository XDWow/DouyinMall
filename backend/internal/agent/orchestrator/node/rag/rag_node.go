package rag

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"

	knowledgebase "github.com/XDWow/DouyinMall/backend/internal/agent/infra/knowledgebase"
)

// Input 描述 RAG 节点真正需要的最小输入。
type Input struct {
	Message string
	History []*schema.Message
	Intent  string
}

// Result 描述 RAG 节点的直接输出。
type Result struct {
	Query     string
	Documents []*schema.Document
}

// RAGNode 负责调用托管知识库完成一次完整的知识检索。
// 当前版本把 RAG 收敛成一个明确节点，而不是在编排层继续拆 rewrite / retrieve / rerank，
// 因为这些步骤已经由托管知识库服务内部完成，上层只需要一个稳定的检索能力边界。
type RAGNode struct {
	KnowledgeBase *knowledgebase.ManagedKnowledgeService
	TopK          int
	MinScore      float64
}

func NewRAGNode(knowledgeBase *knowledgebase.ManagedKnowledgeService, topK int, minScore float64) *RAGNode {
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

	result, err := n.KnowledgeBase.Search(ctx, knowledgebase.SearchInput{
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
