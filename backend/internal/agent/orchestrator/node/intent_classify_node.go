package node

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

// IntentClassifyNode 负责识别用户意图，并补充置信度、实体和是否需要改写。
// 这里不再负责 skill 选择。skill 是否注入由后续编排层按 route 确定，
// 这样可以减少 prompt 体积，也避免把可确定的决策交给概率模型。
type IntentClassifyNode struct {
	Model   model.ToolCallingChatModel
	Prompts *orchestratorprompt.Set
}

func NewIntentClassifyNode(chatModel model.ToolCallingChatModel, prompts *orchestratorprompt.Set) *IntentClassifyNode {
	return &IntentClassifyNode{
		Model:   chatModel,
		Prompts: prompts,
	}
}

type IntentClassifyInput struct {
	Message string
	History []*schema.Message
}

type IntentClassifyResult struct {
	Intent      domain.Intent
	Confidence  float64
	Entities    map[string]string
	NeedRewrite bool
	Reason      string
}

func (n *IntentClassifyNode) Invoke(ctx context.Context, input IntentClassifyInput) (*IntentClassifyResult, error) {
	historyText := support.HistoryText(input.History)
	intent := support.HeuristicIntent(input.Message)
	intent.NeedRewrite = support.RequiresRewrite(input.Message, historyText)

	if n.Model != nil && n.Prompts != nil && n.Prompts.Intent != nil {
		messages, err := n.Prompts.Intent.Format(ctx, map[string]any{
			"system_text":  n.Prompts.SystemText,
			"history_text": normalizeHistoryText(historyText),
			"message":      strings.TrimSpace(input.Message),
		})
		if err == nil {
			msg, genErr := n.Model.Generate(ctx, messages,
				model.WithTemperature(0),
				model.WithMaxTokens(256),
				model.WithToolChoice(schema.ToolChoiceForbidden),
			)
			if genErr == nil && msg != nil {
				if parsed, ok := support.ParseIntentResult(msg.Content); ok {
					intent = parsed
				}
			}
		}
	}

	intent.NeedRewrite = intent.NeedRewrite || support.RequiresRewrite(input.Message, historyText)
	return &IntentClassifyResult{
		Intent:      intent.Intent,
		Confidence:  intent.Confidence,
		Entities:    intent.Entities,
		NeedRewrite: intent.NeedRewrite,
		Reason:      intent.Reason,
	}, nil
}

func normalizeHistoryText(historyText string) string {
	if strings.TrimSpace(historyText) == "" {
		return "none"
	}
	return historyText
}
