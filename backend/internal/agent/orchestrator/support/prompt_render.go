package support

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// HistoryText 把最近消息窗口整理成可直接注入 Prompt 的文本。
func HistoryText(messages []*schema.Message) string {
	if len(messages) == 0 {
		return "none"
	}
	var builder strings.Builder
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		builder.WriteString(string(msg.Role))
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(msg.Content))
		builder.WriteString("\n")
	}
	if builder.Len() == 0 {
		return "none"
	}
	return strings.TrimSpace(builder.String())
}

// DocumentsText 把召回文档整理成适合答案生成的上下文文本。
func DocumentsText(documents []*schema.Document) string {
	if len(documents) == 0 {
		return "none"
	}
	var builder strings.Builder
	for index, doc := range documents {
		if doc == nil {
			continue
		}
		builder.WriteString(fmt.Sprintf(
			"%d. [%s] %s\n",
			index+1,
			FirstNonEmpty(DocumentCategory(doc), "未分类"),
			FirstNonEmpty(DocumentTitle(doc), doc.ID),
		))
		builder.WriteString(DocumentSnippet(doc, 220))
		builder.WriteString("\n")
	}
	if builder.Len() == 0 {
		return "none"
	}
	return strings.TrimSpace(builder.String())
}

// DocumentsToReferences 只在响应边界把文档转换成对外展示用的引用信息。
func DocumentsToReferences(documents []*schema.Document) []domain.KnowledgeRef {
	if len(documents) == 0 {
		return nil
	}
	refs := make([]domain.KnowledgeRef, 0, len(documents))
	for _, doc := range documents {
		if doc == nil {
			continue
		}
		refs = append(refs, domain.KnowledgeRef{
			ID:       doc.ID,
			Title:    DocumentTitle(doc),
			Snippet:  DocumentSnippet(doc, 180),
			Category: DocumentCategory(doc),
			Score:    doc.Score(),
			Metadata: documentMetadata(doc),
		})
	}
	return refs
}

func DocumentTitle(doc *schema.Document) string {
	return documentMetaString(doc, "title")
}

func DocumentCategory(doc *schema.Document) string {
	return documentMetaString(doc, "category")
}

func DocumentSnippet(doc *schema.Document, size int) string {
	if doc == nil {
		return ""
	}
	if snippet := documentMetaString(doc, "snippet"); snippet != "" {
		return Summarize(snippet, size)
	}
	return Summarize(strings.TrimSpace(doc.Content), size)
}

func documentMetaString(doc *schema.Document, key string) string {
	if doc == nil || len(doc.MetaData) == 0 {
		return ""
	}
	value, _ := doc.MetaData[key].(string)
	return strings.TrimSpace(value)
}

func documentMetadata(doc *schema.Document) map[string]string {
	if doc == nil || len(doc.MetaData) == 0 {
		return nil
	}
	out := map[string]string{}
	if nested, ok := doc.MetaData["metadata"].(map[string]string); ok {
		for key, value := range nested {
			out[key] = value
		}
	}
	for key, value := range doc.MetaData {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			out[key] = strings.TrimSpace(text)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
