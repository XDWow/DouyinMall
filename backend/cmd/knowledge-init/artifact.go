package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type KnowledgeArtifact struct {
	Version     string             `json:"version"`
	GeneratedAt time.Time          `json:"generated_at"`
	SourceURL   string             `json:"source_url"`
	KnowledgeID string             `json:"knowledge_id"`
	Title       string             `json:"title"`
	Category    string             `json:"category"`
	Documents   []ArtifactDocument `json:"documents"`
	Chunks      []ArtifactChunk    `json:"chunks"`
}

type ArtifactDocument struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ArtifactChunk struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Snippet  string            `json:"snippet"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func writeArtifact(path string, artifact *KnowledgeArtifact) error {
	if artifact == nil {
		return fmt.Errorf("artifact is nil")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("artifact path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact directory failed: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create artifact file failed: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		return fmt.Errorf("encode artifact failed: %w", err)
	}
	return nil
}

func readArtifact(path string) (*KnowledgeArtifact, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("artifact path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open artifact file failed: %w", err)
	}
	defer file.Close()

	var artifact KnowledgeArtifact
	if err := json.NewDecoder(file).Decode(&artifact); err != nil {
		return nil, fmt.Errorf("decode artifact failed: %w", err)
	}
	if strings.TrimSpace(artifact.KnowledgeID) == "" {
		return nil, fmt.Errorf("artifact knowledge_id is empty")
	}
	if len(artifact.Chunks) == 0 {
		return nil, fmt.Errorf("artifact has no chunks")
	}
	return &artifact, nil
}

func defaultArtifactPath(baseDir, knowledgeID string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "tmp"
	}
	return filepath.Join(baseDir, "knowledge", sanitizeSlug(knowledgeID)+".json")
}
