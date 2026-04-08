package repository

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	rag "github.com/XDWow/DouyinMall/backend/internal/agent/infra/rag"
)

type KnowledgeStore struct {
	dao *DAO
}

func NewKnowledgeStore(dao *DAO) rag.Store {
	return &KnowledgeStore{dao: dao}
}

func (s *KnowledgeStore) TopKByVector(ctx context.Context, vector []float64, limit int) ([]rag.Chunk, error) {
	if limit <= 0 {
		limit = 5
	}

	var rows []KnowledgeChunkDO
	if err := s.dao.db.WithContext(ctx).
		Where("enabled = ?", true).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	chunks := make([]rag.Chunk, 0, len(rows))
	for _, row := range rows {
		var embeddingVec []float64
		if err := json.Unmarshal([]byte(row.Embedding), &embeddingVec); err != nil {
			continue
		}
		score := cosineSimilarity(embeddingVec, vector)
		chunk := rag.Chunk{
			ID:          row.ID,
			KnowledgeID: row.KnowledgeID,
			Title:       row.Title,
			Category:    row.Category,
			Content:     row.Content,
			Snippet:     row.Snippet,
			Score:       score,
			Embedding:   embeddingVec,
		}
		if row.Metadata != "" {
			var metadata map[string]string
			if json.Unmarshal([]byte(row.Metadata), &metadata) == nil {
				chunk.Metadata = metadata
			}
		}
		chunks = append(chunks, chunk)
	}

	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})
	if len(chunks) > limit {
		chunks = chunks[:limit]
	}
	for i := range chunks {
		if chunks[i].Snippet == "" {
			chunks[i].Snippet = summarizeContent(chunks[i].Content)
		}
	}
	return chunks, nil
}

func (s *KnowledgeStore) UpsertChunks(ctx context.Context, chunks []rag.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	rows := make([]KnowledgeChunkDO, 0, len(chunks))
	for _, chunk := range chunks {
		embeddingRaw, err := json.Marshal(chunk.Embedding)
		if err != nil {
			return err
		}
		metadataRaw, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return err
		}
		rows = append(rows, KnowledgeChunkDO{
			ID:          chunk.ID,
			KnowledgeID: chunk.KnowledgeID,
			Title:       chunk.Title,
			Category:    chunk.Category,
			Content:     chunk.Content,
			Snippet:     chunk.Snippet,
			Embedding:   string(embeddingRaw),
			Metadata:    string(metadataRaw),
			Enabled:     true,
		})
	}

	return s.dao.db.WithContext(ctx).
		Save(&rows).Error
}

func summarizeContent(content string) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= 140 {
		return content
	}
	return string(runes[:140]) + "..."
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (mathSqrt(normA) * mathSqrt(normB))
}

func mathSqrt(value float64) float64 {
	// Avoid importing math in multiple files for one function.
	// The compiler will inline this wrapper.
	return sqrt(value)
}
