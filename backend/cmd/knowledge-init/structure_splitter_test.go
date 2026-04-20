package main

import (
	"context"
	"strings"
	"testing"

	recursive "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
)

func TestSplitStructuredDocumentUsesHeadingPath(t *testing.T) {
	t.Parallel()

	doc := &schema.Document{
		ID: "policy-1",
		MetaData: map[string]any{
			"title": "\u4e03\u5929\u65e0\u7406\u7531\u9000\u8d27\u670d\u52a1\u89c4\u8303",
		},
		Content: strings.Join([]string{
			"\u4e03\u5929\u65e0\u7406\u7531\u9000\u8d27\u670d\u52a1\u89c4\u8303",
			"\u7b2c\u4e00\u7ae0 \u6982\u8ff0",
			"1.1 \u76ee\u7684\u53ca\u4f9d\u636e",
			"\u7b2c\u4e00\u6761\u5185\u5bb9",
			"1.2 \u9002\u7528\u8303\u56f4",
			"\u7b2c\u4e8c\u6761\u5185\u5bb9",
			"\u7b2c\u4e8c\u7ae0 \u57fa\u7840\u8981\u6c42",
			"2.1 \u9000\u8d27\u65f6\u6548",
			"\u7b2c\u4e09\u6761\u5185\u5bb9",
		}, "\n"),
	}

	sections := splitStructuredDocument(doc)
	if len(sections) != 3 {
		t.Fatalf("unexpected section count: %d", len(sections))
	}

	firstPath := stringifyMetaValue(sections[0].MetaData["title_path"])
	if firstPath != "\u7b2c\u4e00\u7ae0 \u6982\u8ff0 / 1.1 \u76ee\u7684\u53ca\u4f9d\u636e" {
		t.Fatalf("unexpected first title path: %s", firstPath)
	}
	if !strings.Contains(sections[0].Content, "\u7b2c\u4e00\u7ae0 \u6982\u8ff0") || !strings.Contains(sections[0].Content, "1.1 \u76ee\u7684\u53ca\u4f9d\u636e") {
		t.Fatalf("expected structured headings in content, got: %s", sections[0].Content)
	}

	lastPath := stringifyMetaValue(sections[2].MetaData["title_path"])
	if lastPath != "\u7b2c\u4e8c\u7ae0 \u57fa\u7840\u8981\u6c42 / 2.1 \u9000\u8d27\u65f6\u6548" {
		t.Fatalf("unexpected last title path: %s", lastPath)
	}
}

func TestSplitDocumentsAppliesLengthFallbackWithPrefix(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   60,
		OverlapSize: 10,
		Separators:  []string{"\n\n", "\n", ".", ";"},
		KeepType:    recursive.KeepTypeEnd,
	})
	if err != nil {
		t.Fatalf("init recursive splitter failed: %v", err)
	}

	service := &prepareService{
		splitter:  splitter,
		chunkSize: 60,
	}

	doc := &schema.Document{
		ID: "policy-2",
		Content: strings.Join([]string{
			"\u7b2c\u4e00\u7ae0 \u6982\u8ff0",
			"1.1 \u9000\u8d27\u89c4\u5219",
			"\u6d88\u8d39\u8005\u9700\u5728\u7b7e\u6536\u540e\u4e03\u5929\u5185\u63d0\u4ea4\u9000\u8d27\u7533\u8bf7\u3002",
			"\u5546\u5bb6\u9700\u8981\u6839\u636e\u5e73\u53f0\u89c4\u5219\u53ca\u65f6\u5904\u7406\u5ba1\u6838\u4e0e\u552e\u540e\u65f6\u6548\u3002",
			"\u82e5\u5546\u54c1\u4e0d\u7b26\u5408\u5b8c\u597d\u6807\u51c6\uff0c\u9700\u8981\u5728\u9875\u9762\u4e2d\u6e05\u6670\u63d0\u793a\u9000\u56de\u6761\u4ef6\u3002",
		}, "\n"),
	}

	chunks, err := service.splitDocuments(ctx, []*schema.Document{doc})
	if err != nil {
		t.Fatalf("split documents failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected length fallback to create multiple chunks, got %d", len(chunks))
	}

	for _, chunk := range chunks {
		if got := stringifyMetaValue(chunk.MetaData["title_path"]); got != "\u7b2c\u4e00\u7ae0 \u6982\u8ff0 / 1.1 \u9000\u8d27\u89c4\u5219" {
			t.Fatalf("unexpected title path: %s", got)
		}
		if !strings.Contains(chunk.Content, "\u7b2c\u4e00\u7ae0 \u6982\u8ff0") || !strings.Contains(chunk.Content, "1.1 \u9000\u8d27\u89c4\u5219") {
			t.Fatalf("expected heading prefix in chunk content, got: %s", chunk.Content)
		}
	}
}
