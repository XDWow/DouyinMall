package node

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// ConfirmSummaryInput 描述售后确认节点的输入。
type ConfirmSummaryInput struct {
	Reply  string
	Intent domain.Intent
}

// ConfirmSummaryResult 描述售后确认节点的输出。
type ConfirmSummaryResult struct {
	Reply      string
	Intent     domain.Intent
	Confidence float64
}

// ConfirmSummaryNode 负责把售后确认摘要整理成可中断的响应。
// 它本身只产出结构化结果，真正的中断与状态回写由主图负责。
type ConfirmSummaryNode struct {
	PersistTurn ConversationTurnPersister
	Logger      logger.LoggerV1
}

func NewConfirmSummaryNode(persistTurn ConversationTurnPersister, log logger.LoggerV1) *ConfirmSummaryNode {
	return &ConfirmSummaryNode{
		PersistTurn: persistTurn,
		Logger:      log,
	}
}

// Invoke 生成售后确认阶段的输出。
func (n *ConfirmSummaryNode) Invoke(_ context.Context, input ConfirmSummaryInput) (*ConfirmSummaryResult, error) {
	reply := strings.TrimSpace(input.Reply)
	if reply == "" {
		reply = "请确认是否继续提交售后申请。"
	}
	return &ConfirmSummaryResult{
		Reply:      reply,
		Intent:     input.Intent,
		Confidence: 0.9,
	}, nil
}
