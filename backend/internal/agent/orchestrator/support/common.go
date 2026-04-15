package support

import "strings"

// MergeSlots 把增量槽位合并进目标槽位，只覆盖非空值。
// StringStringMapSnapshot 返回与 src 键值相同的新 map（空 src 返回 nil），用于子图入口与 Session 解耦。
func StringStringMapSnapshot(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func MergeSlots(dst map[string]any, src map[string]any) {
	if dst == nil || len(src) == 0 {
		return
	}
	for key, value := range src {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
		}
		dst[key] = value
	}
}

func IsAffirmative(text string) bool {
	raw := strings.ToLower(strings.TrimSpace(text))
	return ContainsAny(raw, "yes", "y", "ok", "confirm", "sure", "是", "好的", "确认", "确定", "提交")
}

func IsNegative(text string) bool {
	raw := strings.ToLower(strings.TrimSpace(text))
	return ContainsAny(raw, "no", "n", "cancel", "not", "不要", "取消", "算了", "不用")
}

func MentionsInventory(text string) bool {
	raw := strings.ToLower(strings.TrimSpace(text))
	return ContainsAny(raw, "inventory", "stock", "available", "in stock", "库存", "有货", "现货")
}

func AfterSaleTypeLabel(requestType string) string {
	if strings.EqualFold(strings.TrimSpace(requestType), "exchange") {
		return "换货"
	}
	return "退货"
}

func DetectAfterSaleType(text string) string {
	raw := strings.ToLower(strings.TrimSpace(text))
	switch {
	case ContainsAny(raw, "exchange", "replace", "换货", "更换"):
		return "exchange"
	case ContainsAny(raw, "return", "refund", "退货", "退款"):
		return "return"
	default:
		return ""
	}
}

func DetectReturnReason(text string) string {
	raw := strings.TrimSpace(text)
	switch {
	case ContainsAny(strings.ToLower(raw), "broken", "damaged", "defect", "破损", "损坏", "瑕疵"):
		return "商品破损"
	case ContainsAny(strings.ToLower(raw), "size", "fit", "尺寸", "尺码", "不合适"):
		return "尺码不合适"
	case ContainsAny(strings.ToLower(raw), "don't want", "do not want", "不要了", "不想要"):
		return "不想要了"
	default:
		return ""
	}
}

func IsAdvisoryProductInfo(text string) bool {
	raw := strings.ToLower(strings.TrimSpace(text))
	return ContainsAny(raw,
		"how", "which", "compare", "difference", "recommend", "suitable",
		"怎么", "哪个", "区别", "推荐", "适合", "介绍", "说明", "参数",
	)
}
