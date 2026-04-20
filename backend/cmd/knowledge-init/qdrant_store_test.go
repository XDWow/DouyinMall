package main

import (
	"testing"

	qdrant "github.com/qdrant/go-client/qdrant"
)

func TestArtifactChunksToDocuments(t *testing.T) {
	t.Parallel()

	artifact := &KnowledgeArtifact{
		KnowledgeID: "policy_1",
		Title:       "七天无理由退货服务规范",
		Category:    "policy",
		SourceURL:   "https://example.com/policy",
		Chunks: []ArtifactChunk{
			{
				ID:      "policy_1-001",
				Content: "第一章 概述\n1.1 目的及依据\n内容",
				Snippet: "第一章 概述",
				Metadata: map[string]string{
					"title_path": "第一章 概述 / 1.1 目的及依据",
				},
			},
		},
	}

	docs := artifactChunksToDocuments(artifact)
	if len(docs) != 1 {
		t.Fatalf("unexpected docs length: %d", len(docs))
	}
	if got := docs[0].ID; got != "policy_1-001" {
		t.Fatalf("unexpected doc id: %s", got)
	}
	if got := docs[0].MetaData["knowledge_id"]; got != "policy_1" {
		t.Fatalf("unexpected knowledge_id: %#v", got)
	}
	if got := docs[0].MetaData["title_path"]; got != "第一章 概述 / 1.1 目的及依据" {
		t.Fatalf("unexpected title_path: %#v", got)
	}
}

func TestParseQdrantDistance(t *testing.T) {
	t.Parallel()

	if got := parseQdrantDistance("dot_product"); got != qdrant.Distance_Dot {
		t.Fatalf("unexpected dot distance: %v", got)
	}
	if got := parseQdrantDistance("l2"); got != qdrant.Distance_Euclid {
		t.Fatalf("unexpected euclid distance: %v", got)
	}
	if got := parseQdrantDistance(""); got != qdrant.Distance_Cosine {
		t.Fatalf("unexpected default distance: %v", got)
	}
}
