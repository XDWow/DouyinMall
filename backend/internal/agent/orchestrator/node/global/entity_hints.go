package global

import (
	"strings"

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
