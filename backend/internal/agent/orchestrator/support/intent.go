package support

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func RouteFromIntent(intent domain.Intent) graphstate.WorkflowRoute {
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
		return graphstate.RouteFallback
	}
}

func RouteEnabled(flags graphstate.FeatureFlags, route graphstate.WorkflowRoute) bool {
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

func RequiredMissingSlots(state *graphstate.ConversationState) []string {
	switch state.Session.Intent {
	case domain.IntentInventoryQuery, domain.IntentProductInfo, domain.IntentAddToCart:
		return MissingIfEmpty(state, "product_id")
	case domain.IntentReturnExchangeApply:
		if state.Session.AwaitingConfirm {
			return nil
		}
		if missing := MissingIfEmpty(state, "order_id"); len(missing) > 0 {
			return missing
		}
		return MissingIfEmpty(state, "reason")
	default:
		return nil
	}
}

func AskMessageForMissingSlot(state *graphstate.ConversationState, slot string) string {
	switch slot {
	case "order_id":
		if state.Session.Intent == domain.IntentReturnExchangeApply {
			return "请先提供订单号，我再继续为你处理售后申请。"
		}
		return "请提供订单号，我再继续为你处理。"
	case "product_id":
		switch state.Session.Intent {
		case domain.IntentInventoryQuery:
			return "请提供商品 ID 或 SKU，我来帮你查询库存。"
		case domain.IntentAddToCart:
			return "请告诉我你想加入购物车的商品 ID。"
		default:
			return "请提供商品 ID 或 SKU，我再继续为你处理。"
		}
	case "reason":
		return "请告诉我售后原因，例如商品破损、尺码不合适或不想要了。"
	default:
		return "请先补充缺失信息，我再继续为你处理。"
	}
}

func MissingIfEmpty(state *graphstate.ConversationState, keys ...string) []string {
	for _, key := range keys {
		if strings.TrimSpace(graphstate.SlotString(state, key)) == "" {
			return []string{key}
		}
	}
	return nil
}

func HeuristicIntent(message string) graphstate.IntentResult {
	raw := strings.ToLower(strings.TrimSpace(message))
	intent := domain.IntentFallback
	confidence := 0.58

	switch {
	case ContainsAny(raw, "return", "refund", "exchange", "after sale", "退货", "退款", "换货", "售后") &&
		ContainsAny(raw, "apply", "submit", "request", "申请", "提交", "发起"):
		intent, confidence = domain.IntentReturnExchangeApply, 0.92
	case ContainsAny(raw, "return policy", "refund policy", "exchange policy", "7 day", "退换货政策", "退款政策", "换货政策", "七天无理由"):
		intent, confidence = domain.IntentReturnPolicy, 0.86
	case ContainsAny(raw, "add to cart", "add to my cart", "put in cart", "加入购物车", "加购"):
		intent, confidence = domain.IntentAddToCart, 0.90
	case ContainsAny(raw, "inventory", "stock", "available", "in stock", "库存", "有货", "现货"):
		intent, confidence = domain.IntentInventoryQuery, 0.86
	case ContainsAny(raw, "order", "shipping", "delivery", "tracking", "订单", "物流", "发货", "配送"):
		intent, confidence = domain.IntentOrderQuery, 0.90
	case ContainsAny(raw, "product", "price", "spec", "detail", "description", "商品", "价格", "参数", "详情", "介绍"):
		intent, confidence = domain.IntentProductInfo, 0.82
	}

	return graphstate.IntentResult{
		Intent:      intent,
		Confidence:  confidence,
		Entities:    ExtractSimpleEntities(message),
		NeedRewrite: false,
		Reason:      "heuristic",
	}
}

func NormalizeIntent(raw string) domain.Intent {
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

func ParseIntentResult(content string) (graphstate.IntentResult, bool) {
	var payload struct {
		Intent      string            `json:"intent"`
		Confidence  float64           `json:"confidence"`
		NeedRewrite bool              `json:"need_rewrite"`
		Reason      string            `json:"reason"`
		Entities    map[string]string `json:"entities"`
	}
	if err := json.Unmarshal([]byte(CleanJSON(content)), &payload); err != nil {
		return graphstate.IntentResult{}, false
	}
	return graphstate.IntentResult{
		Intent:      NormalizeIntent(payload.Intent),
		Confidence:  Clamp01(payload.Confidence),
		NeedRewrite: payload.NeedRewrite,
		Reason:      payload.Reason,
		Entities:    payload.Entities,
	}, true
}

func ParseRewriteResult(content string) (graphstate.RewriteResult, bool) {
	var payload graphstate.RewriteResult
	if err := json.Unmarshal([]byte(CleanJSON(content)), &payload); err != nil {
		return graphstate.RewriteResult{}, false
	}
	return payload, true
}

// RequiresRewrite 判断当前问题是否需要结合历史上下文做改写。
func RequiresRewrite(message string, historyText string) bool {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return false
	}
	if strings.TrimSpace(historyText) == "" || strings.EqualFold(strings.TrimSpace(historyText), "none") {
		return false
	}
	short := len([]rune(msg)) <= 10
	pronoun := ContainsAny(strings.ToLower(msg), "this", "that", "it", "that one", "这个", "那个", "它")
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

func ExtractSlotsFromMessage(message string, intent domain.Intent) map[string]any {
	result := map[string]any{}
	if id := DigitsOnlyID(message); id != "" {
		switch intent {
		case domain.IntentOrderQuery, domain.IntentReturnExchangeApply:
			result["order_id"] = id
		case domain.IntentInventoryQuery, domain.IntentProductInfo, domain.IntentAddToCart:
			result["product_id"] = id
		}
	}
	if intent == domain.IntentReturnExchangeApply {
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
	case ContainsAny(msg, "damage", "broken", "crack", "破损", "损坏"):
		return "damaged"
	case ContainsAny(msg, "size", "fit", "尺码", "不合适"):
		return "size_issue"
	case ContainsAny(msg, "quality", "defect", "odor", "质量", "瑕疵", "异味"):
		return "quality_issue"
	case ContainsAny(msg, "do not want", "wrong order", "changed my mind", "不想要", "买错"):
		return "personal_reason"
	default:
		return ""
	}
}

func DetectAfterSaleType(message string) string {
	msg := strings.ToLower(message)
	switch {
	case ContainsAny(msg, "exchange", "replace", "换货", "更换"):
		return "exchange"
	case ContainsAny(msg, "return", "refund", "退货", "退款"):
		return "return"
	default:
		return ""
	}
}

func MentionsInventory(text string) bool {
	return ContainsAny(strings.ToLower(text), "inventory", "stock", "available", "in stock", "库存", "有货", "现货")
}

func IsAdvisoryProductInfo(text string) bool {
	return ContainsAny(strings.ToLower(text), "spec", "detail", "price", "recommend", "参数", "详情", "价格", "推荐")
}

func IsAffirmative(text string) bool {
	return ContainsAny(strings.ToLower(strings.TrimSpace(text)), "yes", "ok", "confirm", "sure", "是", "确认", "好的")
}

func IsNegative(text string) bool {
	return ContainsAny(strings.ToLower(strings.TrimSpace(text)), "no", "cancel", "stop", "do not", "否", "取消", "不要")
}

func BuildReturnApplySummary(state *graphstate.ConversationState) string {
	requestType := AfterSaleTypeLabel(FirstNonEmpty(graphstate.SlotString(state, "request_type"), "return"))
	return fmt.Sprintf(
		"请确认是否提交%s申请，订单号 %s，原因：%s。",
		requestType,
		FirstNonEmpty(graphstate.SlotString(state, "order_id"), "未知"),
		FirstNonEmpty(graphstate.SlotString(state, "reason"), "未知"),
	)
}

func AfterSaleTypeLabel(requestType string) string {
	if strings.EqualFold(strings.TrimSpace(requestType), "exchange") {
		return "换货"
	}
	return "退货"
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
