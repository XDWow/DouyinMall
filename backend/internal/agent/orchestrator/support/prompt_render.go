package support

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// HistoryText converts the windowed []*schema.Message slice into a plain-text
// string for injection into prompt templates.  The window has already been
// applied by memory.Manager so no further trimming is needed here.
// Returns "none" when the slice is empty.
func HistoryText(messages []*schema.Message) string {
	if len(messages) == 0 {
		return "none"
	}
	var b strings.Builder
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		b.WriteString(string(msg.Role))
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(msg.Content))
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "none"
	}
	return strings.TrimSpace(b.String())
}

func ReferencesText(refs []domain.KnowledgeRef) string {
	if len(refs) == 0 {
		return "none"
	}
	var builder strings.Builder
	for i, ref := range refs {
		builder.WriteString(fmt.Sprintf("%d. [%s] %s\n%s\n", i+1, ref.Category, FirstNonEmpty(ref.Title, ref.ID), ref.Snippet))
	}
	return strings.TrimSpace(builder.String())
}

func ToolText(execs []domain.ToolExecution) string {
	if len(execs) == 0 {
		return "none"
	}
	var builder strings.Builder
	for i, exec := range execs {
		builder.WriteString(fmt.Sprintf("%d. tool=%s success=%t\n", i+1, exec.Name, exec.Success))
		if strings.TrimSpace(exec.Result) != "" {
			builder.WriteString(exec.Result)
			builder.WriteString("\n")
		}
		if strings.TrimSpace(exec.Error) != "" {
			builder.WriteString("error: ")
			builder.WriteString(exec.Error)
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
}
