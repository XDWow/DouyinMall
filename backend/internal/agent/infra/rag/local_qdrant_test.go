package rag

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

func TestNormalizeQdrantRetrievedDocument(t *testing.T) {
	t.Parallel()

	doc := &schema.Document{
		ID:      "chunk-1",
		Content: "第一章 概述\n内容",
		MetaData: map[string]any{
			"metadata": map[string]*qdrant.Value{
				"title":        qdrant.NewValueString("七天无理由退货服务规范"),
				"knowledge_id": qdrant.NewValueString("policy_1"),
				"title_path":   qdrant.NewValueString("第一章 概述 / 1.1 目的及依据"),
			},
		},
	}
	doc.WithScore(0.88)

	normalized := normalizeQdrantRetrievedDocument(doc)
	if normalized == nil {
		t.Fatal("expected normalized document")
	}
	if got := normalized.MetaData["knowledge_id"]; got != "policy_1" {
		t.Fatalf("unexpected knowledge_id: %#v", got)
	}
	if got := normalized.MetaData["title_path"]; got != "第一章 概述 / 1.1 目的及依据" {
		t.Fatalf("unexpected title_path: %#v", got)
	}
	if normalized.Score() != 0.88 {
		t.Fatalf("unexpected score: %f", normalized.Score())
	}
}

func TestNormalizeQdrantRetrievedDocumentsFiltersByScore(t *testing.T) {
	t.Parallel()

	low := (&schema.Document{ID: "low", Content: "low"}).WithScore(0.2)
	high := (&schema.Document{ID: "high", Content: "high"}).WithScore(0.9)

	docs := normalizeQdrantRetrievedDocuments([]*schema.Document{low, high}, 0.5)
	if len(docs) != 1 || docs[0].ID != "high" {
		t.Fatalf("unexpected filtered docs: %+v", docs)
	}
}
