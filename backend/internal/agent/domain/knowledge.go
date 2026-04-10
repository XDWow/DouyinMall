package domain

type KnowledgeRef struct {
	ID       string
	Title    string
	Snippet  string
	Category string
	Score    float64
	Metadata map[string]string
}
