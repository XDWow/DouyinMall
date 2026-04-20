package support

import "strings"

// MergeSlots 把 UnderstandingNode 新提取出的槽位合并进 Session。
// 只覆盖非空值，避免用户没有明确提到的信息把已有槽位冲掉。
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
