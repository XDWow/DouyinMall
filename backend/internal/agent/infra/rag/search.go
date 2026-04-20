package rag

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

type SearchInput struct {
	Message  string
	History  []*schema.Message
	Intent   string
	TopK     int
	MinScore float64
}

type SearchResult struct {
	Query     string
	Documents []*schema.Document
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
