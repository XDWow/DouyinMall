package domain

import "errors"

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrSearchFailed  = errors.New("search failed")
	ErrLLMFailed     = errors.New("llm call failed")
	ErrEmbeddingFail = errors.New("embedding failed")
)


