package global

import (
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

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

func referenceContext(state *domain.State) string {
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

func routeFromIntent(intent domain.Intent) domain.WorkflowRoute {
	switch intent {
	case domain.IntentOrderQuery:
		return domain.RouteOrderQuery
	case domain.IntentInventoryQuery:
		return domain.RouteInventory
	case domain.IntentProductInfo:
		return domain.RouteProductInfo
	case domain.IntentAddToCart:
		return domain.RouteAddToCart
	case domain.IntentReturnPolicy:
		return domain.RouteReturnPolicy
	case domain.IntentReturnExchangeApply:
		return domain.RouteReturnExchangeApply
	default:
		return domain.RouteBaseQA
	}
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
