package global

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type AskUserInput struct {
	Reply            string
	Intent           domain.Intent
	IntentConfidence float64
	MissingSlots     []string
}

type AskUserNode struct {
}

func NewAskUserNode() *AskUserNode {
	return &AskUserNode{}
}

type AskUserResult struct {
	Reply        string
	Intent       domain.Intent
	Confidence   float64
	MissingSlots []string
}

func (n *AskUserNode) Invoke(_ context.Context, input AskUserInput) (*AskUserResult, error) {
	reply := strings.TrimSpace(input.Reply)
	if reply == "" {
		reply = "\u8bf7\u5148\u8865\u5145\u7f3a\u5931\u4fe1\u606f\uff0c\u6211\u518d\u7ee7\u7eed\u4e3a\u4f60\u5904\u7406\u3002"
	}

	return &AskUserResult{
		Reply:        reply,
		Intent:       input.Intent,
		Confidence:   support.MaxFloat(input.IntentConfidence, 0.8),
		MissingSlots: append([]string(nil), input.MissingSlots...),
	}, nil
}
