package memory

import (
	"context"
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

	var builder strings.Builder
	builder.WriteString("会话摘要：")
	for _, message := range session.Messages {
		role := string(message.Role)
		if role == "" {
			role = "unknown"
		}
		builder.WriteString(role)
		builder.WriteString(":")
		builder.WriteString(strings.TrimSpace(message.Content))
		builder.WriteString("；")
		if builder.Len() >= 320 {
			break
		}
	}
	summary := builder.String()
	runes := []rune(summary)
	if len(runes) <= 320 {
		return summary, nil
	}
	return string(runes[:320]) + "...", nil
}
