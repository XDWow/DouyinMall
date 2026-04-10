package global

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

// IntentAndSlotNode 一次 LLM：意图 + 槽位/实体 + 可选改写；只写 state.Intent / state.Rewrite。
type IntentAndSlotNode struct {
	Model   model.ToolCallingChatModel
	Prompts *orchestratorprompt.Set
}

func NewIntentAndSlotNode(chatModel model.ToolCallingChatModel, prompts *orchestratorprompt.Set) *IntentAndSlotNode {
	return &IntentAndSlotNode{
		Model:   chatModel,
		Prompts: prompts,
	}
}

// Invoke 解析 JSON；失败则简单抽 ID、沿用 Session 意图，保证有 Entities；随后主图 Pre 里会槽位对齐（SlotMergeNode.Apply）。
func (n *IntentAndSlotNode) Invoke(ctx context.Context, state *domain.State) (*domain.State, error) {
	if state == nil {
		return nil, nil
	}

	rawQuery := strings.TrimSpace(state.Session.RawQuery)
	if rawQuery == "" {
		state.Rewrite.Query = ""
		state.Rewrite.Reason = ""
		state.Intent.Intent = domain.IntentUnknown
		state.Intent.Confidence = 0
		state.Intent.Entities = nil
		state.Intent.NeedRewrite = false
		state.Intent.Reason = "empty_message"
		return state, nil
	}

	history := append([]*schema.Message(nil), state.Session.Messages...)
	historyText := support.HistoryText(history)

	// 兜底：可沿用 Session 上轮意图；槽位对齐在 Intent 节点 Pre 的后半段写回 Session。
	state.Intent.Intent = domain.IntentUnknown
	if state.Session.Intent != "" {
		state.Intent.Intent = state.Session.Intent
	}
	state.Intent.Confidence = 0
	state.Intent.Entities = extractSimpleEntities(rawQuery)
	state.Intent.NeedRewrite = false
	state.Intent.Reason = "model_fallback"
	state.Rewrite.Query = rawQuery
	state.Rewrite.Reason = "not_needed"

	if n.Model != nil && n.Prompts != nil && n.Prompts.IntentAndSlot != nil {
		messages, err := n.Prompts.IntentAndSlot.Format(ctx, map[string]any{
			"system_text":       n.Prompts.SystemText,
			"history_text":      normalizeHistoryText(historyText),
			"message":           rawQuery,
			"reference_context": normalizeReferenceContext(referenceContext(state)),
		})
		if err == nil {
			msg, genErr := n.Model.Generate(ctx, messages,
				model.WithTemperature(0),
				model.WithMaxTokens(512),
				model.WithToolChoice(schema.ToolChoiceForbidden),
			)
			if genErr == nil && msg != nil {
				if parsed, ok := parseIntentAndSlotResult(msg.Content); ok {
					slotStr := make(map[string]string, len(parsed.Slots))
					for k, v := range parsed.Slots {
						key := strings.TrimSpace(k)
						if key == "" {
							continue
						}
						slotStr[key] = strings.TrimSpace(fmt.Sprint(v))
					}
					mergedEntities := normalizeModelSlots(slotStr, parsed.Entities)
					intent := normalizeIntent(parsed.Intent)
					needRewrite := parsed.NeedRewrite
					rewriteQuery := rawQuery
					if needRewrite {
						q := strings.TrimSpace(parsed.RewrittenQuery)
						if q != "" {
							rewriteQuery = q
						}
					}
					state.Intent.Intent = intent
					state.Intent.Confidence = support.Clamp01(parsed.Confidence)
					state.Intent.Entities = mergedEntities
					state.Intent.NeedRewrite = needRewrite
					if strings.TrimSpace(parsed.Reason) != "" {
						state.Intent.Reason = strings.TrimSpace(parsed.Reason)
					} else {
						state.Intent.Reason = "model"
					}
					state.Rewrite.Query = rewriteQuery
					if needRewrite && rewriteQuery != rawQuery {
						state.Rewrite.Reason = "model_rewrite"
					} else if needRewrite {
						state.Rewrite.Reason = "model_no_change"
					} else {
						state.Rewrite.Reason = "not_needed"
					}
					return state, nil
				}
			}
		}
	}

	return state, nil
}

type intentAndSlotPayload struct {
	Intent         string            `json:"intent"`
	Confidence     float64           `json:"confidence"`
	NeedRewrite    bool              `json:"need_rewrite"`
	RewrittenQuery string            `json:"rewritten_query"`
	Reason         string            `json:"reason"`
	Entities       map[string]string `json:"entities"`
	Slots          map[string]any    `json:"slots"`
	MissingSlots   []string          `json:"missing_slots"`
}

func parseIntentAndSlotResult(content string) (intentAndSlotPayload, bool) {
	var payload intentAndSlotPayload
	if err := json.Unmarshal([]byte(support.CleanJSON(content)), &payload); err != nil {
		return intentAndSlotPayload{}, false
	}
	if strings.TrimSpace(payload.Intent) == "" {
		return intentAndSlotPayload{}, false
	}
	_ = payload.MissingSlots
	return payload, true
}
