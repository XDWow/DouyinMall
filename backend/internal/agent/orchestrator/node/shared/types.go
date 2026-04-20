package shared

import (
	"fmt"
	"strconv"
	"strings"
)

func SlotString(slots map[string]any, keys ...string) string {
	if len(slots) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := slots[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func ParseSlotInt64(slots map[string]any, keys ...string) (int64, error) {
	raw := SlotString(slots, keys...)
	if raw == "" {
		return 0, fmt.Errorf("missing required slots %v", keys)
	}
	return strconv.ParseInt(raw, 10, 64)
}
