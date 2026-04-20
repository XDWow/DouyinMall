package main

import (
	"context"
	"fmt"
	"strings"

	qdrantindexer "github.com/cloudwego/eino-ext/components/indexer/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

func storeArtifactToQdrant(ctx context.Context, client *qdrant.Client, embedder embedding.Embedder, cfg QdrantConfig, req StoreRequest) error {
	if client == nil {
		return fmt.Errorf("qdrant client is required")
	}
	if embedder == nil {
		return fmt.Errorf("embedder is required")
	}
	if req.Artifact == nil {
		return fmt.Errorf("artifact is required")
	}

	docs := artifactChunksToDocuments(req.Artifact)
	if len(docs) == 0 {
		return fmt.Errorf("artifact has no chunks to store")
	}

	vectorDim, err := resolveQdrantVectorDim(ctx, embedder, cfg.VectorDim, docs)
	if err != nil {
		return err
	}

	indexer, err := qdrantindexer.NewIndexer(ctx, &qdrantindexer.Config{
		Client:     client,
		Collection: strings.TrimSpace(cfg.Collection),
		VectorDim:  vectorDim,
		Distance:   parseQdrantDistance(cfg.Distance),
		BatchSize:  cfg.BatchSize,
		Embedding:  embedder,
	})
	if err != nil {
		return fmt.Errorf("init qdrant indexer failed: %w", err)
	}

	if req.Replace {
		if err := deleteKnowledgeFromQdrant(ctx, client, strings.TrimSpace(cfg.Collection), req.Artifact.KnowledgeID); err != nil {
			return fmt.Errorf("delete existing qdrant points failed: %w", err)
		}
	}

	if _, err := indexer.Store(ctx, docs); err != nil {
		return fmt.Errorf("store knowledge chunks failed: %w", err)
	}
	return nil
}

func artifactChunksToDocuments(artifact *KnowledgeArtifact) []*schema.Document {
	if artifact == nil || len(artifact.Chunks) == 0 {
		return nil
	}

	docs := make([]*schema.Document, 0, len(artifact.Chunks))
	for _, chunk := range artifact.Chunks {
		meta := map[string]any{
			"knowledge_id": artifact.KnowledgeID,
			"title":        artifact.Title,
			"category":     artifact.Category,
			"snippet":      chunk.Snippet,
			"source_url":   artifact.SourceURL,
		}
		for key, value := range chunk.Metadata {
			meta[key] = value
		}
		docs = append(docs, &schema.Document{
			ID:       chunk.ID,
			Content:  chunk.Content,
			MetaData: meta,
		})
	}
	return docs
}

func resolveQdrantVectorDim(ctx context.Context, embedder embedding.Embedder, configured int, docs []*schema.Document) (int, error) {
	if configured > 0 {
		return configured, nil
	}
	if embedder == nil {
		return 0, fmt.Errorf("embedder is required to infer qdrant vector dimension")
	}

	probe := "dimension probe"
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if content := strings.TrimSpace(doc.Content); content != "" {
			probe = content
			break
		}
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{probe})
	if err != nil {
		return 0, fmt.Errorf("infer qdrant vector dimension failed: %w", err)
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return 0, fmt.Errorf("infer qdrant vector dimension returned invalid result")
	}
	return len(vectors[0]), nil
}

func deleteKnowledgeFromQdrant(ctx context.Context, client *qdrant.Client, collection string, knowledgeID string) error {
	collection = strings.TrimSpace(collection)
	knowledgeID = strings.TrimSpace(knowledgeID)
	if collection == "" || knowledgeID == "" {
		return nil
	}

	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewMatch("metadata.knowledge_id", knowledgeID),
		},
	}

	_, err := client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points:         qdrant.NewPointsSelectorFilter(filter),
		Wait:           qdrant.PtrOf(true),
	})
	return err
}

func parseQdrantDistance(raw string) qdrant.Distance {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dot", "dotproduct", "dot_product":
		return qdrant.Distance_Dot
	case "euclid", "euclidean", "l2":
		return qdrant.Distance_Euclid
	case "manhattan", "l1":
		return qdrant.Distance_Manhattan
	default:
		return qdrant.Distance_Cosine
	}
}
