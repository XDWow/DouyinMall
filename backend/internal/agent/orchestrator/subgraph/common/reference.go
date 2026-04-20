package common

import (
	"strconv"
	"strings"
)

func ResolveSelection(ref string, current string, list []string) string {
	raw := strings.TrimSpace(strings.ToLower(ref))
	switch raw {
	case "", "current", "0":
		return strings.TrimSpace(current)
	}
	index, err := strconv.Atoi(raw)
	if err != nil || index <= 0 {
		return ""
	}
	idx := index - 1
	if idx < 0 || idx >= len(list) {
		return ""
	}
	return strings.TrimSpace(list[idx])
}

