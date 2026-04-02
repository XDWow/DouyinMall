//go:build legacy_agent

package domain

import "context"

// SessionRepo 浼氳瘽瀛樺偍锛圧edis 鐑眰 + MySQL 鍐峰眰 + Kafka 寮傛鎸佷箙鍖栵級
type SessionRepo interface {
	// LoadSession 鍔犺浇浼氳瘽鍏冧俊鎭紙浼樺厛 Redis锛宮iss 鍥炴簮 MySQL锛?
	// 涓嶅惈娑堟伅鍒楄〃锛岀敤浜庡垽鏂姸鎬併€佽幏鍙?userID 绛夊厓鏁版嵁鍦烘櫙
	LoadSession(ctx context.Context, sessionID string) (*Session, error)
	// LoadMessages 鍔犺浇浼氳瘽娑堟伅绐楀彛锛堜紭鍏?Redis锛宮iss 鍥炴簮 MySQL锛?
	// 鍙繑鍥炴秷鎭垪琛紝涓嶅惈浼氳瘽鍏冧俊鎭?
	LoadMessages(ctx context.Context, sessionID string) ([]Message, error)
	// AppendMessages 杩藉姞鏈疆鏂版秷鎭細鍐?Redis 鐑眰 + 鎶曢€?Kafka 寮傛钀藉簱
	// newMsgs 鐢辫皟鐢ㄦ柟鏄惧紡浼犲叆锛宺epository 灞備笉鎺ㄦ柇鍝簺娑堟伅鏄柊鐨?
	AppendMessages(ctx context.Context, session *Session, newMsgs []Message) error
	// FlushSession 灏嗕細璇濆厓淇℃伅鍒峰啓鍒?MySQL
	// 浠呭湪浼氳瘽缁堟€侊紙杞汉宸?鍏抽棴锛夋椂璋冪敤锛岃繍琛屾椂 Redis 鏄敮涓€鏉ユ簮
	FlushSession(ctx context.Context, session *Session) error
	// Create 鍒涘缓鏂颁細璇?
	Create(ctx context.Context, session *Session) error
	// Clear 娓呯┖浼氳瘽娑堟伅锛堥噸鏂板紑濮嬶級
	Clear(ctx context.Context, sessionID string) error
	// ListByUser 鑾峰彇鐢ㄦ埛鐨勪細璇濆垪琛紙MySQL 鏌ヨ锛?
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]Session, int, error)
}

type VectorRepo interface {
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]KnowledgeRef, error)
	Insert(ctx context.Context, collection string, id string, vector []float32) error
}

// 涓夊眰缂撳瓨鏋舵瀯
type SemanticCache interface {
	// L1: Exact Cache锛堢簿纭尮閰嶏紝Redis String锛宬ey = "exact:hash(query)"锛?
	ExactLookup(ctx context.Context, query string) (reply string, hit bool, err error)
	ExactStore(ctx context.Context, query, reply string) error

	// L2: Semantic Cache锛堣涔夌浉浼煎害鍖归厤锛屽悜閲忔绱紝鐩镐技搴?>= 0.95 鍛戒腑锛?
	Lookup(ctx context.Context, vector []float32) (reply string, hit bool, err error)
	Store(ctx context.Context, vector []float32, reply string) error

	// L3: RAG Cache锛堢煡璇嗘绱㈢粨鏋滅紦瀛橈紝Redis Hash锛宬ey = "rag:hash(vector)"锛?
	// 缂撳瓨 query vector 鈫?knowledge refs 鐨勬槧灏勶紝閬垮厤閲嶅妫€绱?
	// 閫傜敤鍦烘櫙锛氬洖澶嶈川閲忎笉绋冲畾锛屼絾鐭ヨ瘑搴撴绱㈢粨鏋滃彲澶嶇敤
	RAGLookup(ctx context.Context, vector []float32) (knowledge []KnowledgeRef, hit bool, err error)
	RAGStore(ctx context.Context, vector []float32, knowledge []KnowledgeRef) error
}
