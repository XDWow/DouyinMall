package global

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

type QueryRewriteNode struct {
	Model   model.ToolCallingChatModel
	Prompts *orchestratorprompt.Set
}

func NewQueryRewriteNode(chatModel model.ToolCallingChatModel, prompts *orchestratorprompt.Set) *QueryRewriteNode {
	return &QueryRewriteNode{
		Model:   chatModel,
		Prompts: prompts,
	}
}

func (n *QueryRewriteNode) Invoke(ctx context.Context, state *graphstate.State) (*graphstate.State, error) {
	if state == nil {
		return nil, nil
	}

	rawQuery := strings.TrimSpace(state.Session.RawQuery)
	if rawQuery == "" {
		state.Rewrite.Query = ""
		state.Rewrite.Reason = ""
		return state, nil
	}

	historyText := support.HistoryText(append([]*schema.Message(nil), state.Session.Messages...))
	if !requiresRewrite(rawQuery, historyText) {
		state.Rewrite.Query = rawQuery
		state.Rewrite.Reason = "not_needed"
		return state, nil
	}

	rewritten := rawQuery
	reason := "prompt_fallback"
	intent := domain.IntentUnknown
	if state.Session.Intent != "" {
		intent = state.Session.Intent
	}

	if n.Model != nil && n.Prompts != nil && n.Prompts.Rewrite != nil {
		messages, err := n.Prompts.Rewrite.Format(ctx, map[string]any{
			"system_text":  n.Prompts.SystemText,
			"history_text": normalizeHistoryText(historyText),
			"message":      rawQuery,
			"intent":       string(intent),
		})
		if err == nil {
			msg, genErr := n.Model.Generate(ctx, messages,
				model.WithTemperature(0),
				model.WithMaxTokens(256),
				model.WithToolChoice(schema.ToolChoiceForbidden),
			)
			if genErr == nil && msg != nil {
				if parsed, ok := parseRewriteResult(msg.Content); ok && strings.TrimSpace(parsed.Query) != "" {
					rewritten = strings.TrimSpace(parsed.Query)
					reason = strings.TrimSpace(parsed.Reason)
				}
			}
		}
	}

	state.Rewrite.Query = rewritten
	state.Rewrite.Reason = reason
	return state, nil
}
