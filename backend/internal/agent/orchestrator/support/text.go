package support

import (
	"fmt"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`\d{4,}`)

func SplitTerms(text string) []string {
	text = strings.ToLower(text)
	replacer := strings.NewReplacer(
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"?", " ",
		"!", " ",
		"\n", " ",
		"\t", " ",
	)
	return strings.Fields(replacer.Replace(text))
}

func KeywordOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]bool, len(a))
	for _, item := range a {
		set[item] = true
	}
	var hit int
	for _, item := range b {
		if set[item] {
			hit++
		}
	}
	return float64(hit) / float64(len(a))
}

func Summarize(content string, size int) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= size {
		return content
	}
	return string(runes[:size]) + "..."
}

func Clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func MaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func ToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func DigitsOnlyID(raw string) string {
	return idPattern.FindString(raw)
}

func CleanJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}

func ContainsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
