package support

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	"github.com/XDWow/DouyinMall/backend/internal/agent/memory"
)

func HistoryMessages(session *memory.Session, limit int) []*schema.Message {
	if session == nil {
		return nil
	}
	items := session.RecentMessages(limit)
	out := make([]*schema.Message, 0, len(items))
	for _, item := range items {
		switch item.Role {
		case dto.RoleAssistant:
			out = append(out, schema.AssistantMessage(item.Content, nil))
		case dto.RoleSystem:
			out = append(out, schema.SystemMessage(item.Content))
		case dto.RoleTool:
			out = append(out, schema.ToolMessage(item.Content, item.ID))
		default:
			out = append(out, schema.UserMessage(item.Content))
		}
	}
	return out
}

func HistoryText(session *memory.Session, limit int) string {
	if session == nil {
		return "none"
	}
	var builder strings.Builder
	for _, item := range session.RecentMessages(limit) {
		builder.WriteString(string(item.Role))
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(item.Content))
		builder.WriteString("\n")
	}
	if builder.Len() == 0 {
		return "none"
	}
	return strings.TrimSpace(builder.String())
}

func ReferencesText(refs []dto.KnowledgeRef) string {
	if len(refs) == 0 {
		return "none"
	}
	var builder strings.Builder
	for i, ref := range refs {
		builder.WriteString(fmt.Sprintf("%d. [%s] %s\n%s\n", i+1, ref.Category, FirstNonEmpty(ref.Title, ref.ID), ref.Snippet))
	}
	return strings.TrimSpace(builder.String())
}

func ToolText(execs []dto.ToolExecution) string {
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
