package global

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// AskUserInput 缺参追问（interrupt 带 missing_slots）。
type AskUserInput struct {
	Reply            string
	Intent           domain.Intent
	IntentConfidence float64
	MissingSlots     []string
}

// AskUserNode 统一缺参问句与置信度下限。
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
		reply = "请补充缺失信息。"
	}

	return &AskUserResult{
		Reply:        reply,
		Intent:       input.Intent,
		Confidence:   support.MaxFloat(input.IntentConfidence, 0.8),
		MissingSlots: append([]string(nil), input.MissingSlots...),
	}, nil
}
