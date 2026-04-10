package global

import (
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

// RequiredMissingSlots 按意图返回仍未满足的业务必填槽位键名；仅子图内调用，主图不做全局门禁。
// entities 与 slots 一起看过没过必填；可与解析结果对照，不要求先写进 Session.Slots。
func RequiredMissingSlots(intent domain.Intent, slots map[string]any, entities map[string]string, awaitingConfirm bool) []string {
	if entities == nil {
		entities = map[string]string{}
	}
	entity := func(key string) string {
		return strings.TrimSpace(entities[key])
	}
	switch intent {
	case domain.IntentInventoryQuery, domain.IntentProductInfo, domain.IntentAddToCart:
		has := strings.TrimSpace(orchestratorshared.SlotString(slots, "product_id", "sku_id")) != ""
		if !has {
			has = entity("product_id") != "" || entity("sku_id") != ""
		}
		if !has {
			return []string{"product_id"}
		}
	case domain.IntentReturnExchangeApply:
		if awaitingConfirm {
			return nil
		}
		if strings.TrimSpace(orchestratorshared.SlotString(slots, "order_id")) == "" && entity("order_id") == "" {
			return []string{"order_id"}
		}
		if strings.TrimSpace(orchestratorshared.SlotString(slots, "reason")) == "" && entity("reason") == "" {
			return []string{"reason"}
		}
	}
	return nil
}

// AskMessageForMissingSlot 针对首个缺失槽位生成给用户看的追问文案。
func AskMessageForMissingSlot(intent domain.Intent, slot string) string {
	switch slot {
	case "order_id":
		if intent == domain.IntentReturnExchangeApply {
			return "请先提供订单号，我再继续为你处理售后申请。"
		}
		return "请提供订单号，我再继续为你处理。"
	case "product_id":
		switch intent {
		case domain.IntentInventoryQuery:
			return "请提供商品 ID 或 SKU，我来帮你查库存。"
		case domain.IntentAddToCart:
			return "请告诉我你想加入购物车的商品 ID。"
		default:
			return "请提供商品 ID 或 SKU，我再继续为你处理。"
		}
	case "reason":
		return "请告诉我售后原因，比如商品破损、尺码不合适或不想要了。"
	default:
		return "请先补充缺失信息，我再继续为你处理。"
	}
}
