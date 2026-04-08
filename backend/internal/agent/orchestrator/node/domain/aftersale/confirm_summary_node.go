package aftersale

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type ConfirmSummaryInput struct {
	Reply  string
	Intent domain.Intent
}

type ConfirmSummaryResult struct {
	Reply      string
	Intent     domain.Intent
	Confidence float64
}

type ConfirmSummaryNode struct {
}

func NewConfirmSummaryNode() *ConfirmSummaryNode {
	return &ConfirmSummaryNode{}
}

func (n *ConfirmSummaryNode) Invoke(_ context.Context, input ConfirmSummaryInput) (*ConfirmSummaryResult, error) {
	reply := strings.TrimSpace(input.Reply)
	if reply == "" {
		reply = "\u8bf7\u786e\u8ba4\u662f\u5426\u63d0\u4ea4\u672c\u6b21\u552e\u540e\u7533\u8bf7\u3002"
	}
	return &ConfirmSummaryResult{
		Reply:      reply,
		Intent:     input.Intent,
		Confidence: 0.9,
	}, nil
}
