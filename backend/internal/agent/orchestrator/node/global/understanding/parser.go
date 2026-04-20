package understanding

import (
	"encoding/json"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type llmUnderstandingPayload struct {
	Intent         string         `json:"intent"`
	RewrittenQuery string         `json:"rewritten_query"`
	Slots          map[string]any `json:"slots"`
}

func ParseUnderstandingResult(content string) *UnderstandingResult {
	res, ok := ParseUnderstandingResultWithOK(content)
	if !ok {
		return fallbackUnderstandingResult()
	}
	return res
}

func ParseUnderstandingResultWithOK(content string) (*UnderstandingResult, bool) {
	cleaned := support.CleanJSON(strings.TrimSpace(content))
	if cleaned == "" {
		return nil, false
	}

	var payload llmUnderstandingPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return nil, false
	}

	intent := normalizeIntentString(payload.Intent)
	if intent == "" {
		return nil, false
	}

	return &UnderstandingResult{
		Intent:         intent,
		RewrittenQuery: strings.TrimSpace(payload.RewrittenQuery),
		Slots:          payload.Slots,
	}, true
}

func fallbackUnderstandingResult() *UnderstandingResult {
	return &UnderstandingResult{
		Intent:         IntentUnknown,
		RewrittenQuery: "",
		Slots:          nil,
	}
}

func normalizeIntentString(raw string) Intent {
	switch Intent(strings.TrimSpace(strings.ToLower(raw))) {
	case IntentProductService:
		return IntentProductService
	case IntentOrderService:
		return IntentOrderService
	case IntentPromotionService:
		return IntentPromotionService
	case IntentAftersalesPolicy:
		return IntentAftersalesPolicy
	case IntentAftersalesApply:
		return IntentAftersalesApply
	case IntentAddToCart:
		return IntentAddToCart
	case IntentUnknown:
		return IntentUnknown
	default:
		return ""
	}
}
