//go:build legacy_agent

package domain

// RAG 鍛戒腑鐨勭煡璇嗗紩鐢紙杩斿洖缁欒皟鐢ㄦ柟锛?
type KnowledgeRef struct {
	ID        string
	Title     string
	Content   string // chunk 瀹屾暣鍐呭锛堝凡鍒嗙墖锛屾棤闇€鎴柇锛?
	Category  string
	Relevance float32 // 鐩稿叧鎬у緱鍒?
}

// 鍚戦噺闆嗗悎鍚嶏紙domain 瀹氫箟锛宨nfra 浣跨敤锛寀secase 浼犲弬锛?
const (
	CollectionKnowledge = "agent_knowledge"
	CollectionCache     = "agent_semantic_cache"
)
