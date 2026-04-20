package main

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"
)

var (
	chineseHeadingPattern  = regexp.MustCompile(`^(第[零〇一二三四五六七八九十百千万两0-9]+(?:编|部分|卷|篇|章|节|条))(?:\s+(.+))?$`)
	numberedHeadingPattern = regexp.MustCompile(`^(\d+(?:\.\d+){1,5})(?:\s+)?(.+)$`)
)

type structuredHeading struct {
	Number string
	Title  string
	Kind   string
	Level  int
}

type structuredSection struct {
	Headings []structuredHeading
	Body     []string
}

func (s *prepareService) splitDocuments(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	sections := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		sections = append(sections, splitStructuredDocument(doc)...)
	}
	if len(sections) == 0 {
		return nil, nil
	}

	chunks := make([]*schema.Document, 0, len(sections))
	for _, section := range sections {
		if section == nil || !needsLengthSplit(section.Content, s.chunkSize) {
			if section != nil {
				chunks = append(chunks, section)
			}
			continue
		}

		splitDocs, err := s.splitter.Transform(ctx, []*schema.Document{section})
		if err != nil {
			return nil, err
		}
		splitDocs = normalizeDocuments(splitDocs)
		headerPrefix := structuredHeaderPrefix(section.MetaData)
		for _, chunk := range splitDocs {
			if chunk == nil {
				continue
			}
			chunk.MetaData = mergeMetadata(section.MetaData, chunk.MetaData)
			chunk.Content = normalizeExtractedText(applyHeaderPrefix(headerPrefix, chunk.Content))
			if strings.TrimSpace(chunk.Content) == "" {
				continue
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

func splitStructuredDocument(doc *schema.Document) []*schema.Document {
	if doc == nil || strings.TrimSpace(doc.Content) == "" {
		return nil
	}

	lines := dropLeadingTitleLine(strings.Split(doc.Content, "\n"), metadataTitleFromDocument(doc))
	sections := make([]*schema.Document, 0, 4)
	var preamble []string
	var stack []structuredHeading
	var current *structuredSection

	flushCurrent := func(next *structuredHeading) {
		if current == nil || len(current.Headings) == 0 {
			return
		}

		body := normalizeExtractedText(strings.Join(current.Body, "\n"))
		if body == "" && next != nil && next.Level > current.Headings[len(current.Headings)-1].Level {
			current = nil
			return
		}
		if body == "" {
			current = nil
			return
		}

		meta := mergeMetadata(nil, doc.MetaData)
		if meta == nil {
			meta = map[string]any{}
		}
		addStructuredMetadata(meta, current.Headings)
		meta["chunk_strategy"] = "structure_then_recursive"

		sections = append(sections, &schema.Document{
			ID:       doc.ID,
			Content:  buildStructuredSectionContent(current.Headings, body),
			MetaData: meta,
		})
		current = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			if current != nil && len(current.Body) > 0 && current.Body[len(current.Body)-1] != "" {
				current.Body = append(current.Body, "")
			} else if current == nil && len(preamble) > 0 && preamble[len(preamble)-1] != "" {
				preamble = append(preamble, "")
			}
			continue
		}

		if heading, ok := detectStructuredHeading(line); ok {
			flushCurrent(&heading)
			stack = updateHeadingStack(stack, heading)
			current = &structuredSection{
				Headings: cloneHeadings(stack),
				Body:     append([]string(nil), preamble...),
			}
			preamble = nil
			continue
		}

		if current == nil {
			preamble = append(preamble, line)
			continue
		}
		current.Body = append(current.Body, line)
	}
	flushCurrent(nil)

	if len(sections) > 0 {
		return sections
	}

	content := normalizeExtractedText(strings.Join(preamble, "\n"))
	if content == "" {
		content = normalizeExtractedText(doc.Content)
	}
	if content == "" {
		return nil
	}

	return []*schema.Document{{
		ID:       doc.ID,
		Content:  content,
		MetaData: mergeMetadata(nil, doc.MetaData),
	}}
}

func detectStructuredHeading(line string) (structuredHeading, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return structuredHeading{}, false
	}

	if matches := chineseHeadingPattern.FindStringSubmatch(line); len(matches) > 0 {
		number := strings.TrimSpace(matches[1])
		kind := chineseHeadingKind(number)
		return structuredHeading{
			Number: number,
			Title:  line,
			Kind:   kind,
			Level:  chineseHeadingLevel(kind),
		}, true
	}

	if matches := numberedHeadingPattern.FindStringSubmatch(line); len(matches) > 0 {
		number := strings.TrimSpace(matches[1])
		return structuredHeading{
			Number: number,
			Title:  line,
			Kind:   "numbered",
			Level:  len(strings.Split(number, ".")),
		}, true
	}

	return structuredHeading{}, false
}

func updateHeadingStack(stack []structuredHeading, heading structuredHeading) []structuredHeading {
	if heading.Level <= 0 {
		return []structuredHeading{heading}
	}

	trimmed := append([]structuredHeading(nil), stack...)
	if len(trimmed) >= heading.Level {
		trimmed = append([]structuredHeading(nil), trimmed[:heading.Level-1]...)
	}
	trimmed = append(trimmed, heading)
	return trimmed
}

func cloneHeadings(headings []structuredHeading) []structuredHeading {
	if len(headings) == 0 {
		return nil
	}
	cloned := make([]structuredHeading, len(headings))
	copy(cloned, headings)
	return cloned
}

func buildStructuredSectionContent(headings []structuredHeading, body string) string {
	lines := make([]string, 0, len(headings)+1)
	for _, heading := range headings {
		if title := strings.TrimSpace(heading.Title); title != "" {
			lines = append(lines, title)
		}
	}
	if text := strings.TrimSpace(body); text != "" {
		lines = append(lines, text)
	}
	return normalizeExtractedText(strings.Join(lines, "\n"))
}

func addStructuredMetadata(meta map[string]any, headings []structuredHeading) {
	if meta == nil || len(headings) == 0 {
		return
	}

	pathTitles := make([]string, 0, len(headings))
	pathNumbers := make([]string, 0, len(headings))
	for idx, heading := range headings {
		if heading.Title != "" {
			pathTitles = append(pathTitles, heading.Title)
		}
		if heading.Number != "" {
			pathNumbers = append(pathNumbers, heading.Number)
		}
		switch idx {
		case 0:
			meta["chapter_title"] = heading.Title
		case 1:
			meta["section_title"] = heading.Title
		case 2:
			meta["clause_title"] = heading.Title
		case 3:
			meta["subclause_title"] = heading.Title
		}
	}

	last := headings[len(headings)-1]
	meta["title_path"] = strings.Join(pathTitles, " / ")
	meta["heading_title"] = last.Title
	meta["heading_kind"] = last.Kind
	meta["heading_level"] = strconv.Itoa(last.Level)
	if last.Number != "" {
		meta["section_number"] = last.Number
	}
	if len(pathNumbers) > 0 {
		meta["heading_number_path"] = strings.Join(pathNumbers, " / ")
	}
}

func structuredHeaderPrefix(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}

	lines := make([]string, 0, 4)
	for _, key := range []string{"chapter_title", "section_title", "clause_title", "subclause_title"} {
		if title := stringifyMetaValue(meta[key]); title != "" {
			lines = append(lines, title)
		}
	}
	if len(lines) == 0 {
		if title := stringifyMetaValue(meta["heading_title"]); title != "" {
			lines = append(lines, title)
		}
	}
	return strings.Join(lines, "\n")
}

func applyHeaderPrefix(prefix, content string) string {
	prefix = strings.TrimSpace(prefix)
	content = strings.TrimSpace(content)
	switch {
	case prefix == "":
		return content
	case content == "":
		return prefix
	case strings.HasPrefix(content, prefix):
		return content
	default:
		return prefix + "\n" + content
	}
}

func dropLeadingTitleLine(lines []string, title string) []string {
	title = strings.TrimSpace(title)
	if title == "" {
		return lines
	}

	for idx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == title {
			return append([]string(nil), lines[idx+1:]...)
		}
		break
	}
	return lines
}

func metadataTitleFromDocument(doc *schema.Document) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	for _, key := range []string{"title", "_title", "name"} {
		if title := stringifyMetaValue(doc.MetaData[key]); title != "" {
			return title
		}
	}
	return ""
}

func needsLengthSplit(content string, limit int) bool {
	if limit <= 0 {
		return false
	}
	return len([]rune(strings.TrimSpace(content))) > limit
}

func mergeMetadata(base map[string]any, overlay map[string]any) map[string]any {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	merged := make(map[string]any, len(base)+len(overlay))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overlay {
		if _, exists := merged[key]; !exists {
			merged[key] = value
		}
	}
	return merged
}

func chineseHeadingKind(number string) string {
	for _, suffix := range []string{"部分", "编", "卷", "篇", "章", "节", "条"} {
		if strings.HasSuffix(number, suffix) {
			return suffix
		}
	}
	return "chapter"
}

func chineseHeadingLevel(kind string) int {
	switch kind {
	case "部分", "编", "卷", "篇", "章":
		return 1
	case "节":
		return 2
	case "条":
		return 3
	default:
		return 1
	}
}
