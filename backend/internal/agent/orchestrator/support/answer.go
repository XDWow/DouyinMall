package support

import (
	"fmt"
	"strings"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func FallbackAnswer(state *graphstate.ConversationState) string {
	if state != nil && len(state.Retrieval.References) > 0 {
		ref := state.Retrieval.References[0]
		return fmt.Sprintf("According to knowledge base [%s], %s", FirstNonEmpty(ref.Title, "reference"), ref.Snippet)
	}
	return "I do not have enough information yet. Please retry later or hand off to a human agent."
}

func TemplateAnswer(state *graphstate.ConversationState) string {
	if state == nil {
		return "I need a bit more context before I can continue. Please provide the order ID or product information."
	}
	if strings.TrimSpace(state.Session.FinalAnswer) != "" {
		return state.Session.FinalAnswer
	}

	switch state.Session.Route {
	case graphstate.RouteOrderQuery:
		if result := ToolResultMap(state, "query_order"); len(result) > 0 {
			if orders, ok := result["orders"].([]any); ok && len(orders) > 0 {
				first, _ := orders[0].(map[string]any)
				if len(orders) == 1 && first != nil {
					return fmt.Sprintf("Order %v is currently in status %v.", first["order_id"], first["status"])
				}
				return fmt.Sprintf("I found %d related orders. Tell me which one you want to inspect.", len(orders))
			}
		}
	case graphstate.RouteInventory:
		if result := ToolResultMap(state, "get_inventory"); len(result) > 0 {
			if stock, ok := result["available_stock"]; ok {
				return fmt.Sprintf("Available inventory is %v.", stock)
			}
		}
	case graphstate.RouteProductInfo:
		if result := ToolResultMap(state, "get_product"); len(result) > 0 {
			if product, ok := result["product"].(map[string]any); ok {
				return fmt.Sprintf("%v is priced at %v and sold by %v.", product["name"], product["price"], product["merchant_name"])
			}
		}
	}
	return FallbackAnswer(state)
}

func NormalizeReply(reply string) string {
	reply = strings.TrimSpace(reply)
	reply = strings.Trim(reply, "\"")
	return reply
}

func EstimateConfidence(state *graphstate.ConversationState) float64 {
	score := Clamp01(state.Session.IntentConfidence) * 0.5
	if len(state.Retrieval.References) > 0 {
		score += Clamp01(state.Retrieval.References[0].Score) * 0.2
	}
	successTools := 0
	for _, exec := range state.ToolExecutions() {
		if exec.Success {
			successTools++
		}
	}
	if successTools > 0 {
		score += 0.2
	}
	if strings.TrimSpace(state.Session.FinalAnswer) != "" || strings.TrimSpace(state.Answer.Reply) != "" {
		score += 0.1
	}
	if state.Session.NeedHandoff {
		score -= 0.25
	}
	return Clamp01(score)
}

func ShouldUseLLMAnswer(state *graphstate.ConversationState) bool {
	if state == nil || state.Session.NeedHandoff {
		return false
	}
	switch state.Session.Route {
	case graphstate.RouteReturnPolicy, graphstate.RouteFallback:
		return len(state.Retrieval.References) > 0
	case graphstate.RouteProductInfo:
		return len(state.Retrieval.References) > 0 || len(state.ToolExecutions()) > 0
	case graphstate.RouteOrderQuery, graphstate.RouteInventory:
		return len(state.ToolExecutions()) > 0
	default:
		return false
	}
}

