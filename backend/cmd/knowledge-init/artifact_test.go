package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestArtifactRoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "artifact.json")
	artifact := &KnowledgeArtifact{
		Version:     "v1",
		GeneratedAt: time.Now().UTC().Round(0),
		SourceURL:   "https://example.com/doc",
		KnowledgeID: "example_doc",
		Title:       "Example",
		Category:    "policy",
		Documents: []ArtifactDocument{
			{ID: "doc-1", Content: "cleaned text", Metadata: map[string]string{"title": "Example"}},
		},
		Chunks: []ArtifactChunk{
			{ID: "example_doc-001", Content: "chunk text", Snippet: "chunk text", Metadata: map[string]string{"chunk_index": "1"}},
		},
	}

	if err := writeArtifact(path, artifact); err != nil {
		t.Fatalf("write artifact failed: %v", err)
	}

	loaded, err := readArtifact(path)
	if err != nil {
		t.Fatalf("read artifact failed: %v", err)
	}
	if loaded.KnowledgeID != artifact.KnowledgeID {
		t.Fatalf("unexpected knowledge id: %s", loaded.KnowledgeID)
	}
	if len(loaded.Chunks) != 1 || loaded.Chunks[0].ID != "example_doc-001" {
		t.Fatalf("unexpected chunks: %+v", loaded.Chunks)
	}
}

func TestDefaultArtifactPath(t *testing.T) {
	t.Parallel()

	path := defaultArtifactPath("tmp", "jinritemai_article_101835")
	expected := filepath.Join("tmp", "knowledge", "jinritemai_article_101835.json")
	if path != expected {
		t.Fatalf("unexpected artifact path: %s", path)
	}
}
