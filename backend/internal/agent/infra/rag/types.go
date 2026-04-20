package rag

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type Chunk struct {
	ID          string
	KnowledgeID string
	Title       string
	Category    string
	Content     string
	Snippet     string
	Score       float64
	Metadata    map[string]string
	Embedding   []float64
}

type Store interface {
	TopKByVector(ctx context.Context, vector []float64, limit int) ([]Chunk, error)
	UpsertChunks(ctx context.Context, chunks []Chunk) error
}

type Searcher interface {
	Search(ctx context.Context, input SearchInput) (*SearchResult, error)
}

type Chunker interface {
	Split(content string) []string
}

func ToDocument(chunk Chunk) *schema.Document {
	doc := &schema.Document{
		ID:      chunk.ID,
		Content: chunk.Content,
		MetaData: map[string]any{
			"title":        chunk.Title,
			"category":     chunk.Category,
			"knowledge_id": chunk.KnowledgeID,
			"snippet":      chunk.Snippet,
			"metadata":     chunk.Metadata,
		},
	}
	return doc.WithScore(chunk.Score)
}
