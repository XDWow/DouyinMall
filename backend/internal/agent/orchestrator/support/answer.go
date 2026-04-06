package support

import (
	"fmt"
	"strings"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func FallbackAnswer(state *graphstate.ConversationState) string {
	if state != nil && len(state.Retrieval.Documents) > 0 {
		doc := state.Retrieval.Documents[0]
		return fmt.Sprintf(
			"根据知识库《%s》，%s",
			FirstNonEmpty(DocumentTitle(doc), "参考资料"),
			DocumentSnippet(doc, 180),
		)
	}
	return "我暂时还缺少足够信息，请稍后重试或转人工处理。"
}

func TemplateAnswer(state *graphstate.ConversationState) string {
	if state == nil {
		return "我还需要更多上下文信息，请提供订单号或商品信息。"
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
					return fmt.Sprintf("订单 %v 当前状态为 %v。", first["order_id"], first["status"])
				}
				return fmt.Sprintf("我找到了 %d 个相关订单，请告诉我你想查看哪一个。", len(orders))
			}
		}
	case graphstate.RouteInventory:
		if result := ToolResultMap(state, "get_inventory"); len(result) > 0 {
			if stock, ok := result["available_stock"]; ok {
				return fmt.Sprintf("当前可售库存为 %v。", stock)
			}
		}
	case graphstate.RouteProductInfo:
		if result := ToolResultMap(state, "get_product"); len(result) > 0 {
			if product, ok := result["product"].(map[string]any); ok {
				return fmt.Sprintf("%v 当前售价 %v，商家为 %v。", product["name"], product["price"], product["merchant_name"])
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
	if len(state.Retrieval.Documents) > 0 {
		score += Clamp01(state.Retrieval.Documents[0].Score()) * 0.2
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
		return len(state.Retrieval.Documents) > 0
	case graphstate.RouteProductInfo:
		return len(state.Retrieval.Documents) > 0 || len(state.ToolExecutions()) > 0
	case graphstate.RouteOrderQuery, graphstate.RouteInventory:
		return len(state.ToolExecutions()) > 0
	default:
		return false
	}
}
