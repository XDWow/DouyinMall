package domain

import "context"

// SessionRepo 会话存储（Redis 热层 + MySQL 冷层 + Kafka 异步持久化）
type SessionRepo interface {
	// LoadSession 加载会话元信息（优先 Redis，miss 回源 MySQL）
	// 不含消息列表，用于判断状态、获取 userID 等元数据场景
	LoadSession(ctx context.Context, sessionID string) (*Session, error)
	// LoadMessages 加载会话消息窗口（优先 Redis，miss 回源 MySQL）
	// 只返回消息列表，不含会话元信息
	LoadMessages(ctx context.Context, sessionID string) ([]Message, error)
	// AppendMessages 追加本轮新消息：写 Redis 热层 + 投递 Kafka 异步落库
	// newMsgs 由调用方显式传入，repository 层不推断哪些消息是新的
	AppendMessages(ctx context.Context, session *Session, newMsgs []Message) error
	// FlushSession 将会话元信息刷写到 MySQL
	// 仅在会话终态（转人工/关闭）时调用，运行时 Redis 是唯一来源
	FlushSession(ctx context.Context, session *Session) error
	// Create 创建新会话
	Create(ctx context.Context, session *Session) error
	// Clear 清空会话消息（重新开始）
	Clear(ctx context.Context, sessionID string) error
	// ListByUser 获取用户的会话列表（MySQL 查询）
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]Session, int, error)
}

type VectorRepo interface {
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]KnowledgeRef, error)
	Insert(ctx context.Context, collection string, id string, vector []float32) error
}

// 三层缓存架构
type SemanticCache interface {
	// L1: Exact Cache（精确匹配，Redis String，key = "exact:hash(query)"）
	ExactLookup(ctx context.Context, query string) (reply string, hit bool, err error)
	ExactStore(ctx context.Context, query, reply string) error

	// L2: Semantic Cache（语义相似度匹配，向量检索，相似度 >= 0.95 命中）
	Lookup(ctx context.Context, vector []float32) (reply string, hit bool, err error)
	Store(ctx context.Context, vector []float32, reply string) error

	// L3: RAG Cache（知识检索结果缓存，Redis Hash，key = "rag:hash(vector)"）
	// 缓存 query vector → knowledge refs 的映射，避免重复检索
	// 适用场景：回复质量不稳定，但知识库检索结果可复用
	RAGLookup(ctx context.Context, vector []float32) (knowledge []KnowledgeRef, hit bool, err error)
	RAGStore(ctx context.Context, vector []float32, knowledge []KnowledgeRef) error
}
