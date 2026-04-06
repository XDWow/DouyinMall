package node

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// AskUserInput 描述补槽追问阶段的输入。
type AskUserInput struct {
	Reply            string
	Intent           domain.Intent
	IntentConfidence float64
	MissingSlots     []string
}

// AskUserNode 负责把补槽追问整理成可中断的响应。
type AskUserNode struct {
	PersistTurn ConversationTurnPersister
	Logger      logger.LoggerV1
}

func NewAskUserNode(persistTurn ConversationTurnPersister, log logger.LoggerV1) *AskUserNode {
	return &AskUserNode{
		PersistTurn: persistTurn,
		Logger:      log,
	}
}

type AskUserResult struct {
	Reply        string
	Intent       domain.Intent
	Confidence   float64
	MissingSlots []string
}

// Invoke 生成需要返回给用户的补槽提问。
func (n *AskUserNode) Invoke(_ context.Context, input AskUserInput) (*AskUserResult, error) {
	reply := strings.TrimSpace(input.Reply)
	if reply == "" {
		reply = "请先补充缺失信息，我再继续为你处理。"
	}

	return &AskUserResult{
		Reply:        reply,
		Intent:       input.Intent,
		Confidence:   support.MaxFloat(input.IntentConfidence, 0.8),
		MissingSlots: append([]string(nil), input.MissingSlots...),
	}, nil
}
