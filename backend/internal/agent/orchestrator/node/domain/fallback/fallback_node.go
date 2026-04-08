package fallback

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type BaseQANode struct{}

func NewBaseQANode() *BaseQANode { return &BaseQANode{} }

type BaseQAInput struct {
	FinalAnswer string
	Documents   []*schema.Document
}

type BaseQAResult struct {
	FinalAnswer string
}

func (n *BaseQANode) Invoke(_ context.Context, input BaseQAInput) (*BaseQAResult, error) {
	answer := strings.TrimSpace(input.FinalAnswer)
	if answer == "" {
		answer = support.BaseQAAnswerFromDocuments(input.Documents)
	}
	return &BaseQAResult{FinalAnswer: answer}, nil
}
