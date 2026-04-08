package global

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

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
	Message          string
	History          []*schema.Message
	ReferenceContext string
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
	intent := graphstate.IntentResult{
		Intent:      domain.IntentUnknown,
		Confidence:  0,
		Entities:    extractSimpleEntities(input.Message),
		NeedRewrite: requiresRewrite(input.Message, historyText),
		Reason:      "model_required",
	}

	if n.Model != nil && n.Prompts != nil && n.Prompts.Intent != nil {
		messages, err := n.Prompts.Intent.Format(ctx, map[string]any{
			"system_text":       n.Prompts.SystemText,
			"history_text":      normalizeHistoryText(historyText),
			"message":           strings.TrimSpace(input.Message),
			"reference_context": normalizeReferenceContext(input.ReferenceContext),
		})
		if err == nil {
			msg, genErr := n.Model.Generate(ctx, messages,
				model.WithTemperature(0),
				model.WithMaxTokens(256),
				model.WithToolChoice(schema.ToolChoiceForbidden),
			)
			if genErr == nil && msg != nil {
				if parsed, ok := parseIntentResult(msg.Content); ok {
					intent = parsed
				}
			}
		}
	}

	intent.NeedRewrite = intent.NeedRewrite || requiresRewrite(input.Message, historyText)
	return &IntentClassifyResult{
		Intent:      intent.Intent,
		Confidence:  intent.Confidence,
		Entities:    intent.Entities,
		NeedRewrite: intent.NeedRewrite,
		Reason:      intent.Reason,
	}, nil
}

func (n *IntentClassifyNode) Apply(ctx context.Context, state *graphstate.State) (*graphstate.State, error) {
	if state == nil {
		return nil, nil
	}

	message := strings.TrimSpace(firstNonEmpty(state.Rewrite.Query, state.Session.RawQuery))
	result, err := n.Invoke(ctx, IntentClassifyInput{
		Message:          message,
		History:          append([]*schema.Message(nil), state.Session.Messages...),
		ReferenceContext: referenceContext(state),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return state, nil
	}

	state.Intent.Intent = result.Intent
	state.Intent.Confidence = result.Confidence
	state.Intent.Entities = result.Entities
	state.Intent.NeedRewrite = result.NeedRewrite
	state.Intent.Reason = result.Reason
	state.Session.Intent = result.Intent
	state.Session.IntentConfidence = result.Confidence
	return state, nil
}

func normalizeHistoryText(historyText string) string {
	if strings.TrimSpace(historyText) == "" {
		return "none"
	}
	return historyText
}

func normalizeReferenceContext(referenceContext string) string {
	if strings.TrimSpace(referenceContext) == "" {
		return "none"
	}
	return referenceContext
}

func routeFromIntent(intent domain.Intent) graphstate.WorkflowRoute {
	switch intent {
	case domain.IntentOrderQuery:
		return graphstate.RouteOrderQuery
	case domain.IntentInventoryQuery:
		return graphstate.RouteInventory
	case domain.IntentProductInfo:
		return graphstate.RouteProductInfo
	case domain.IntentAddToCart:
		return graphstate.RouteAddToCart
	case domain.IntentReturnPolicy:
		return graphstate.RouteReturnPolicy
	case domain.IntentReturnExchangeApply:
		return graphstate.RouteReturnExchangeApply
	default:
		return graphstate.RouteBaseQA
	}
}

func routeEnabled(flags graphstate.FeatureFlags, route graphstate.WorkflowRoute) bool {
	switch route {
	case graphstate.RouteOrderQuery:
		return flags.OrderQuery
	case graphstate.RouteInventory:
		return flags.Inventory
	case graphstate.RouteProductInfo:
		return flags.ProductInfo
	case graphstate.RouteAddToCart:
		return flags.AddToCart
	case graphstate.RouteReturnPolicy:
		return flags.ReturnPolicy
	case graphstate.RouteReturnExchangeApply:
		return flags.ReturnExchangeApply
	default:
		return true
	}
}

func parseIntentResult(content string) (graphstate.IntentResult, bool) {
	var payload struct {
		Intent      string            `json:"intent"`
		Confidence  float64           `json:"confidence"`
		NeedRewrite bool              `json:"need_rewrite"`
		Reason      string            `json:"reason"`
		Entities    map[string]string `json:"entities"`
		Slots       map[string]string `json:"slots"`
	}
	if err := json.Unmarshal([]byte(support.CleanJSON(content)), &payload); err != nil {
		return graphstate.IntentResult{}, false
	}
	return graphstate.IntentResult{
		Intent:      normalizeIntent(payload.Intent),
		Confidence:  support.Clamp01(payload.Confidence),
		NeedRewrite: payload.NeedRewrite,
		Reason:      payload.Reason,
		Entities:    normalizeModelSlots(payload.Slots, payload.Entities),
	}, true
}

func parseRewriteResult(content string) (graphstate.RewriteResult, bool) {
	var payload graphstate.RewriteResult
	if err := json.Unmarshal([]byte(support.CleanJSON(content)), &payload); err != nil {
		return graphstate.RewriteResult{}, false
	}
	return payload, true
}

func requiresRewrite(message string, historyText string) bool {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return false
	}
	if strings.TrimSpace(historyText) == "" || strings.EqualFold(strings.TrimSpace(historyText), "none") {
		return false
	}
	short := len([]rune(msg)) <= 10
	pronoun := support.ContainsAny(strings.ToLower(msg), "this", "that", "it", "that one", "这个", "那个", "它")
	return short || pronoun
}

func normalizeIntent(raw string) domain.Intent {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "order_query":
		return domain.IntentOrderQuery
	case "inventory", "inventory_query":
		return domain.IntentInventoryQuery
	case "product_info", "product_search":
		return domain.IntentProductInfo
	case "add_to_cart", "cart":
		return domain.IntentAddToCart
	case "return_policy", "policy":
		return domain.IntentReturnPolicy
	case "return_exchange_apply", "apply_return_exchange", "apply_return":
		return domain.IntentReturnExchangeApply
	case "fallback", "faq", "unknown", "unsupported":
		return domain.IntentFallback
	default:
		return domain.IntentUnknown
	}
}

func extractSimpleEntities(message string) map[string]string {
	entities := map[string]string{}
	if id := support.DigitsOnlyID(message); id != "" {
		entities["id"] = id
	}
	return entities
}

func extractMetadataSlots(metadata map[string]string) map[string]any {
	result := map[string]any{}
	for key, value := range metadata {
		switch key {
		case "order_id", "orderID":
			if id := support.DigitsOnlyID(value); id != "" {
				result["order_id"] = id
			}
		case "product_id", "productID":
			if id := support.DigitsOnlyID(value); id != "" {
				result["product_id"] = id
			}
		case "sku_id", "skuID":
			if id := support.DigitsOnlyID(value); id != "" {
				result["sku_id"] = id
			}
		case "reason":
			if strings.TrimSpace(value) != "" {
				result["reason"] = strings.TrimSpace(value)
			}
		}
	}
	return result
}

func normalizeEntitySlots(entities map[string]string) map[string]any {
	result := map[string]any{}
	for key, value := range entities {
		switch key {
		case "reason":
			result[key] = strings.TrimSpace(value)
		case "request_type":
			if normalized := support.DetectAfterSaleType(value); normalized != "" {
				result[key] = normalized
			}
		case "quantity":
			if normalized := normalizeQuantity(value); normalized != "" {
				result[key] = normalized
			}
		case "product_ref", "order_ref":
			if normalized := normalizeRefHint(value); normalized != "" {
				result[key] = normalized
			}
		}
	}
	return result
}

func extractSafeSlotsFromMessage(message string, intent domain.Intent) map[string]any {
	result := map[string]any{}
	if intent == domain.IntentReturnExchangeApply {
		if requestType := support.DetectAfterSaleType(message); requestType != "" {
			result["request_type"] = requestType
		}
		if reason := support.DetectReturnReason(message); reason != "" {
			result["reason"] = reason
		}
	}
	return result
}

func normalizeModelSlots(slots map[string]string, entities map[string]string) map[string]string {
	if len(slots) == 0 {
		return entities
	}
	merged := make(map[string]string, len(entities)+len(slots))
	for key, value := range entities {
		merged[key] = value
	}
	for key, value := range slots {
		merged[key] = value
	}
	return merged
}

func normalizeRefHint(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "current", "current_product", "current_order":
		return value
	default:
		return ""
	}
}

func normalizeQuantity(raw string) string {
	value := support.DigitsOnlyID(raw)
	if value == "" {
		return ""
	}
	return value
}

func referenceContext(state *graphstate.State) string {
	if state == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if state.Session.CurrentRefs.ProductID != "" {
		parts = append(parts, "current_product=available")
	}
	if state.Session.CurrentRefs.OrderID != "" {
		parts = append(parts, "current_order=available")
	}
	return strings.Join(parts, "\n")
}
