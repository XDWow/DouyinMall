package global

import (
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// ApplyIntentFieldsForTools：意图解析出的键 → 子图里当轮 map，只用来调工具；写 Session.Slots 仍走工具成功回写。
func ApplyIntentFieldsForTools(slots map[string]any, intentFields map[string]string) {
	if slots == nil || len(intentFields) == 0 {
		return
	}
	for _, key := range []string{"order_id", "product_id", "sku_id", "reason"} {
		if strings.TrimSpace(orchestratorshared.SlotString(slots, key)) != "" {
			continue
		}
		val := strings.TrimSpace(intentFields[key])
		if val == "" {
			continue
		}
		switch key {
		case "order_id", "product_id", "sku_id":
			if id := support.DigitsOnlyID(val); id != "" {
				slots[key] = id
			}
		default:
			slots[key] = val
		}
	}
}

// ResolveOrderRefFromTrustedRefs：模型在意图 JSON 的 entities 里输出指代（如 order_ref=current），
// 数字 order_id 由本函数用会话 CurrentRefs（工具 hydrate / 持久化的「当前单」）解析进 slots，供下游规则节点调单。
func ResolveOrderRefFromTrustedRefs(slots map[string]any, intentFields map[string]string, refs domain.CurrentRefs) {
	if slots == nil || len(intentFields) == 0 {
		return
	}
	if strings.TrimSpace(orchestratorshared.SlotString(slots, "order_id")) != "" {
		return
	}
	ref := strings.ToLower(strings.TrimSpace(intentFields["order_ref"]))
	if ref == "" {
		return
	}
	switch ref {
	case "current", "this":
		if id := support.DigitsOnlyID(refs.OrderID); id != "" {
			slots["order_id"] = id
		}
	}
}
