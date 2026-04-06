package node

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type FallbackNode struct{}

func NewFallbackNode() *FallbackNode { return &FallbackNode{} }

// FallbackInput 描述兜底节点真正依赖的最小输入。
type FallbackInput struct {
	FinalAnswer string
	Documents   []*schema.Document
}

// FallbackResult 表示兜底阶段输出的最终文案。
type FallbackResult struct {
	FinalAnswer string
}

// Invoke 生成兜底阶段的状态更新结果。
func (n *FallbackNode) Invoke(_ context.Context, input FallbackInput) (*FallbackResult, error) {
	answer := strings.TrimSpace(input.FinalAnswer)
	if answer == "" {
		answer = support.FallbackAnswerFromDocuments(input.Documents)
	}
	return &FallbackResult{FinalAnswer: answer}, nil
}
