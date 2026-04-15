package support

import (
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func BaseQAAnswer(st *domain.State) string {
	if st != nil && len(st.Retrieval.Documents) > 0 {
		doc := st.Retrieval.Documents[0]
		return fmt.Sprintf(
			"根据知识库《%s》，%s",
			FirstNonEmpty(DocumentTitle(doc), "参考资料"),
			DocumentSnippet(doc, 180),
		)
	}
	return "我暂时还缺少足够信息，请稍后重试或转人工处理。"
}

func TemplateAnswer(st *domain.State) string {
	if st == nil {
		return "我还需要更多上下文信息，请提供订单号或商品信息。"
	}
	if strings.TrimSpace(st.Session.FinalAnswer) != "" {
		return st.Session.FinalAnswer
	}

	switch st.Session.Route {
	case domain.RouteOrderQuery:
		if r := ToolResultMap(st, "get_order"); len(r) > 0 {
			if ord, ok := r["order"].(map[string]any); ok && ord != nil {
				return fmt.Sprintf("订单 %v 当前状态为 %v。", ord["order_id"], ord["status"])
			}
		}
		result := ToolResultMap(st, "list_user_orders")
		if len(result) == 0 {
			result = ToolResultMap(st, "query_order")
		}
		if len(result) > 0 {
			if orders, ok := result["orders"].([]any); ok && len(orders) > 0 {
				first, _ := orders[0].(map[string]any)
				if len(orders) == 1 && first != nil {
					return fmt.Sprintf("订单 %v 当前状态为 %v。", first["order_id"], first["status"])
				}
				return fmt.Sprintf("我找到了 %d 个相关订单，请告诉我你想查看哪一个。", len(orders))
			}
		}
	case domain.RouteInventory:
		if result := ToolResultMap(st, "get_inventory"); len(result) > 0 {
			if stock, ok := result["available_stock"]; ok {
				return fmt.Sprintf("当前可售库存为 %v。", stock)
			}
		}
	case domain.RouteProductInfo:
		if result := ToolResultMap(st, "get_product"); len(result) > 0 {
			if product, ok := result["product"].(map[string]any); ok {
				return fmt.Sprintf("%v 当前售价 %v，商家为 %v。", product["name"], product["price"], product["merchant_name"])
			}
		}
	}
	return BaseQAAnswer(st)
}

func NormalizeReply(reply string) string {
	reply = strings.TrimSpace(reply)
	reply = strings.Trim(reply, "\"")
	return reply
}

func EstimateConfidence(st *domain.State) float64 {
	if st == nil {
		return 0
	}
	score := Clamp01(st.Session.IntentConfidence) * 0.5
	if len(st.Retrieval.Documents) > 0 {
		score += Clamp01(st.Retrieval.Documents[0].Score()) * 0.2
	}
	successTools := 0
	for _, exec := range st.ToolExecutions() {
		if exec.Success {
			successTools++
		}
	}
	if successTools > 0 {
		score += 0.2
	}
	if strings.TrimSpace(st.Session.FinalAnswer) != "" || strings.TrimSpace(st.Answer.Reply) != "" {
		score += 0.1
	}
	if st.Session.NeedHandoff {
		score -= 0.25
	}
	return Clamp01(score)
}
