package rag

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type VectorRetriever struct {
	store      Store
	embedder   embedding.Embedder
	defaultTop int
}

func NewVectorRetriever(store Store, embedder embedding.Embedder, defaultTop int) *VectorRetriever {
	if defaultTop <= 0 {
		defaultTop = 8
	}
	return &VectorRetriever{
		store:      store,
		embedder:   embedder,
		defaultTop: defaultTop,
	}
}

func (r *VectorRetriever) Retrieve(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	options := einoretriever.GetCommonOptions(&einoretriever.Options{}, opts...)
	topK := r.defaultTop
	if options.TopK != nil && *options.TopK > 0 {
		topK = *options.TopK
	}

	embedder := r.embedder
	if options.Embedding != nil {
		embedder = options.Embedding
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, nil
	}

	chunks, err := r.store.TopKByVector(ctx, vectors[0], topK)
	if err != nil {
		return nil, err
	}

	documents := make([]*schema.Document, 0, len(chunks))
	for _, chunk := range chunks {
		if options.ScoreThreshold != nil && chunk.Score < *options.ScoreThreshold {
			continue
		}
		documents = append(documents, ToDocument(chunk))
	}
	return documents, nil
}

