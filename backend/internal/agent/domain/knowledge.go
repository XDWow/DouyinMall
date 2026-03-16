package domain

// RAG 命中的知识引用（返回给调用方）
type KnowledgeRef struct {
	ID        string
	Title     string
	Content   string // chunk 完整内容（已分片，无需截断）
	Category  string
	Relevance float32 // 相关性得分
}

// 向量集合名（domain 定义，infra 使用，usecase 传参）
const (
	CollectionKnowledge = "agent_knowledge"
	CollectionCache     = "agent_semantic_cache"
)
