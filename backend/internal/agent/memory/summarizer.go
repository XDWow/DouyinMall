package memory

import (
	"context"
	"fmt"
	"strings"
)

type FallbackSummarizer struct{}

func NewFallbackSummarizer() Summarizer {
	return &FallbackSummarizer{}
}

func (s *FallbackSummarizer) Summarize(_ context.Context, session *Session) (string, error) {
	if session == nil || len(session.Messages) == 0 {
		return "", nil
	}

	const limit = 320

	var builder strings.Builder
	builder.WriteString("Conversation summary: ")

	for _, message := range session.Messages {
		role := string(message.Role)
		if role == "" {
			role = "unknown"
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}

		builder.WriteString(fmt.Sprintf("[%s] %s; ", role, content))
		if builder.Len() >= limit {
			break
		}
	}

	summary := builder.String()
	runes := []rune(summary)
	if len(runes) <= limit {
		return summary, nil
	}
	return string(runes[:limit]) + "...", nil
}
