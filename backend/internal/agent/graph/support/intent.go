package support

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
)

func RouteFromIntent(intent dto.Intent) graphstate.WorkflowRoute {
	switch intent {
	case dto.IntentOrderQuery:
		return graphstate.RouteOrderQuery
	case dto.IntentReturnPolicy:
		return graphstate.RouteReturnPolicy
	case dto.IntentInventoryQuery:
		return graphstate.RouteInventory
	case dto.IntentProductInfo:
		return graphstate.RouteProductInfo
	case dto.IntentReturnExchangeApply:
		return graphstate.RouteReturnExchangeApply
	default:
		return graphstate.RouteFallback
	}
}

func RouteEnabled(flags graphstate.FeatureFlags, route graphstate.WorkflowRoute) bool {
	switch route {
	case graphstate.RouteOrderQuery:
		return flags.OrderQuery
	case graphstate.RouteReturnPolicy:
		return flags.ReturnPolicy
	case graphstate.RouteInventory:
		return flags.Inventory
	case graphstate.RouteProductInfo:
		return flags.ProductInfo
	case graphstate.RouteReturnExchangeApply:
		return flags.ReturnExchangeApply
	default:
		return true
	}
}

func RequiredMissingSlots(flow *graphstate.FlowContext) []string {
	switch flow.State.Intent {
	case dto.IntentInventoryQuery, dto.IntentProductInfo:
		return MissingIfEmpty(flow, "product_id")
	case dto.IntentReturnExchangeApply:
		if flow.State.AwaitingConfirm {
			return nil
		}
		if missing := MissingIfEmpty(flow, "order_id"); len(missing) > 0 {
			return missing
		}
		return MissingIfEmpty(flow, "reason")
	default:
		return nil
	}
}

func AskMessageForMissingSlot(flow *graphstate.FlowContext, slot string) string {
	switch slot {
	case "order_id":
		if flow.State.Intent == dto.IntentReturnExchangeApply {
			return "Please provide the order ID before I continue the after-sale request."
		}
		return "Please provide the order ID so I can continue."
	case "product_id":
		if flow.State.Intent == dto.IntentInventoryQuery {
			return "Please provide the product ID or SKU so I can check inventory."
		}
		return "Please provide the product ID or SKU so I can continue."
	case "reason":
		return "Please tell me the after-sale reason, for example damage, wrong size, or changed your mind."
	default:
		return "Please provide the missing information so I can continue."
	}
}

func MissingIfEmpty(flow *graphstate.FlowContext, keys ...string) []string {
	for _, key := range keys {
		if strings.TrimSpace(graphstate.SlotString(flow, key)) == "" {
			return []string{key}
		}
	}
	return nil
}

func HeuristicIntent(message string) graphstate.IntentDecision {
	raw := strings.ToLower(strings.TrimSpace(message))
	intent := dto.IntentFallback
	confidence := 0.58

	switch {
	case ContainsAny(raw, "return", "refund", "exchange", "after sale") && ContainsAny(raw, "apply", "submit", "request"):
		intent, confidence = dto.IntentReturnExchangeApply, 0.92
	case ContainsAny(raw, "return policy", "refund policy", "exchange policy", "7 day"):
		intent, confidence = dto.IntentReturnPolicy, 0.86
	case ContainsAny(raw, "inventory", "stock", "available", "in stock"):
		intent, confidence = dto.IntentInventoryQuery, 0.86
	case ContainsAny(raw, "order", "shipping", "delivery", "tracking"):
		intent, confidence = dto.IntentOrderQuery, 0.90
	case ContainsAny(raw, "product", "price", "spec", "detail", "description"):
		intent, confidence = dto.IntentProductInfo, 0.82
	}

	return graphstate.IntentDecision{Intent: intent, Confidence: confidence, Entities: ExtractSimpleEntities(message), NeedRewrite: false, Reason: "heuristic"}
}

func NormalizeIntent(raw string) dto.Intent {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "order_query":
		return dto.IntentOrderQuery
	case "return_policy", "policy":
		return dto.IntentReturnPolicy
	case "inventory", "inventory_query":
		return dto.IntentInventoryQuery
	case "product_info", "product_search":
		return dto.IntentProductInfo
	case "return_exchange_apply", "apply_return_exchange", "apply_return":
		return dto.IntentReturnExchangeApply
	case "fallback", "faq", "unknown", "unsupported":
		return dto.IntentFallback
	default:
		return dto.IntentUnknown
	}
}

func ParseIntentDecision(content string) (graphstate.IntentDecision, bool) {
	var payload struct {
		Intent      string            `json:"intent"`
		Confidence  float64           `json:"confidence"`
		NeedRewrite bool              `json:"need_rewrite"`
		Reason      string            `json:"reason"`
		Entities    map[string]string `json:"entities"`
	}
	if err := json.Unmarshal([]byte(CleanJSON(content)), &payload); err != nil {
		return graphstate.IntentDecision{}, false
	}
	return graphstate.IntentDecision{Intent: NormalizeIntent(payload.Intent), Confidence: Clamp01(payload.Confidence), NeedRewrite: payload.NeedRewrite, Reason: payload.Reason, Entities: payload.Entities}, true
}

func ParseRewriteDecision(content string) (string, string, bool) {
	var payload struct {
		Query  string `json:"query"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(CleanJSON(content)), &payload); err != nil {
		return "", "", false
	}
	return payload.Query, payload.Reason, true
}

func RequiresRewrite(message string, session *memory.Session) bool {
	msg := strings.TrimSpace(message)
	if msg == "" || session == nil || len(session.Messages) == 0 {
		return false
	}
	short := len([]rune(msg)) <= 10
	pronoun := ContainsAny(strings.ToLower(msg), "this", "that", "it", "that one")
	return short || pronoun
}

func ExtractSimpleEntities(message string) map[string]string {
	entities := map[string]string{}
	if id := DigitsOnlyID(message); id != "" {
		entities["id"] = id
	}
	return entities
}

func ExtractMetadataSlots(metadata map[string]string) map[string]any {
	result := map[string]any{}
	for key, value := range metadata {
		switch key {
		case "order_id", "orderID":
			if id := DigitsOnlyID(value); id != "" {
				result["order_id"] = id
			}
		case "product_id", "productID":
			if id := DigitsOnlyID(value); id != "" {
				result["product_id"] = id
			}
		case "sku_id", "skuID":
			if id := DigitsOnlyID(value); id != "" {
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

func NormalizeEntitySlots(entities map[string]string) map[string]any {
	result := map[string]any{}
	for key, value := range entities {
		switch key {
		case "order_id", "product_id", "sku_id", "reason":
			result[key] = value
		case "id":
			if strings.TrimSpace(value) != "" {
				result["id"] = value
			}
		}
	}
	return result
}

func ExtractSlotsFromMessage(message string, intent dto.Intent) map[string]any {
	result := map[string]any{}
	if id := DigitsOnlyID(message); id != "" {
		switch intent {
		case dto.IntentOrderQuery, dto.IntentReturnExchangeApply:
			result["order_id"] = id
		case dto.IntentInventoryQuery, dto.IntentProductInfo:
			result["product_id"] = id
		}
	}
	if intent == dto.IntentReturnExchangeApply {
		if requestType := DetectAfterSaleType(message); requestType != "" {
			result["request_type"] = requestType
		}
		if reason := DetectReturnReason(message); reason != "" {
			result["reason"] = reason
		}
	}
	return result
}

func MergeSlots(dst map[string]any, src map[string]any) {
	if dst == nil || len(src) == 0 {
		return
	}
	for key, value := range src {
		if value != nil && strings.TrimSpace(ToString(value)) != "" {
			dst[key] = value
		}
	}
}

func DetectReturnReason(message string) string {
	msg := strings.ToLower(message)
	switch {
	case ContainsAny(msg, "damage", "broken", "crack"):
		return "damaged"
	case ContainsAny(msg, "size", "fit"):
		return "size_issue"
	case ContainsAny(msg, "quality", "defect", "odor"):
		return "quality_issue"
	case ContainsAny(msg, "do not want", "wrong order", "changed my mind"):
		return "personal_reason"
	default:
		return ""
	}
}

func DetectAfterSaleType(message string) string {
	msg := strings.ToLower(message)
	switch {
	case ContainsAny(msg, "exchange", "replace"):
		return "exchange"
	case ContainsAny(msg, "return", "refund"):
		return "return"
	default:
		return ""
	}
}

func MentionsInventory(text string) bool {
	return ContainsAny(strings.ToLower(text), "inventory", "stock", "available", "in stock")
}
func IsAdvisoryProductInfo(text string) bool {
	return ContainsAny(strings.ToLower(text), "spec", "detail", "price", "recommend")
}
func IsAffirmative(text string) bool {
	return ContainsAny(strings.ToLower(strings.TrimSpace(text)), "yes", "ok", "confirm", "sure")
}
func IsNegative(text string) bool {
	return ContainsAny(strings.ToLower(strings.TrimSpace(text)), "no", "cancel", "stop", "do not")
}

func BuildReturnApplySummary(flow *graphstate.FlowContext) string {
	requestType := AfterSaleTypeLabel(FirstNonEmpty(graphstate.SlotString(flow, "request_type"), "return"))
	return fmt.Sprintf("Please confirm the %s request for order %s with reason %s.", requestType, FirstNonEmpty(graphstate.SlotString(flow, "order_id"), "unknown"), FirstNonEmpty(graphstate.SlotString(flow, "reason"), "unknown"))
}

func AfterSaleTypeLabel(requestType string) string {
	if strings.EqualFold(strings.TrimSpace(requestType), "exchange") {
		return "exchange"
	}
	return "return"
}

func MetadataValue(metadata map[string]string, keys ...string) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}
