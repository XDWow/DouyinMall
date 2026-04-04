//go:build legacy_agent

package domain

// RAG 閸涙垝鑵戦惃鍕叀鐠囧棗绱╅悽顭掔礄鏉╂柨娲栫紒娆掔殶閻劍鏌熼敍?
type KnowledgeRef struct {
	ID        string
	Title     string
	Content   string // chunk 鐎瑰本鏆ｉ崘鍛啇閿涘牆鍑￠崚鍡欏閿涘本妫ら棁鈧幋顏呮焽閿?
	Category  string
	Relevance float32 // 閻╃鍙ч幀褍绶遍崚?
}

// 閸氭垿鍣洪梿鍡楁値閸氬稄绱檇omain 鐎规矮绠熼敍瀹╪fra 娴ｈ法鏁ら敍瀵€secase 娴肩姴寮敍?
const (
	CollectionKnowledge = "agent_knowledge"
	CollectionCache     = "agent_semantic_cache"
)


