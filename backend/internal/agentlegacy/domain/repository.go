//go:build legacy_agent

package domain

import "context"

// SessionRepo 娴兼俺鐦界€涙ê鍋嶉敍鍦dis 閻戭厼鐪?+ MySQL 閸愬嘲鐪?+ Kafka 瀵倹顒為幐浣风畽閸栨牭绱?type SessionRepo interface {
	// LoadSession 閸旂姾娴囨导姘崇樈閸忓啩淇婇幁顖ょ礄娴兼ê鍘?Redis閿涘iss 閸ョ偞绨?MySQL閿?
	// 娑撳秴鎯堝☉鍫熶紖閸掓銆冮敍宀€鏁ゆ禍搴″灲閺傤厾濮搁幀浣碘偓浣藉箯閸?userID 缁涘鍘撻弫鐗堝祦閸︾儤娅?	LoadSession(ctx context.Context, sessionID string) (*Session, error)
	// LoadMessages 閸旂姾娴囨导姘崇樈濞戝牊浼呯粣妤€褰涢敍鍫滅喘閸?Redis閿涘iss 閸ョ偞绨?MySQL閿?
	// 閸欘亣绻戦崶鐐寸Х閹垰鍨悰顭掔礉娑撳秴鎯堟导姘崇樈閸忓啩淇婇幁?
	LoadMessages(ctx context.Context, sessionID string) ([]Message, error)
	// AppendMessages 鏉╄棄濮為張顒冪枂閺傜増绉烽幁顖ょ窗閸?Redis 閻戭厼鐪?+ 閹舵洟鈧?Kafka 瀵倹顒為拃钘夌氨
	// newMsgs 閻㈣精鐨熼悽銊︽煙閺勬儳绱℃导鐘插弳閿涘epository 鐏炲倷绗夐幒銊︽焽閸濐亙绨哄☉鍫熶紖閺勵垱鏌婇惃?
	AppendMessages(ctx context.Context, session *Session, newMsgs []Message) error
	// FlushSession 鐏忓棔绱扮拠婵嗗帗娣団剝浼呴崚宄板晸閸?MySQL
	// 娴犲懎婀导姘崇樈缂佸牊鈧緤绱欐潪顑挎眽瀹?閸忔娊妫撮敍澶嬫鐠嬪啰鏁ら敍宀冪箥鐞涘本妞?Redis 閺勵垰鏁稉鈧弶銉︾爱
	FlushSession(ctx context.Context, session *Session) error
	// Create 閸掓稑缂撻弬棰佺窗鐠?
	Create(ctx context.Context, session *Session) error
	// Clear 濞撳懐鈹栨导姘崇樈濞戝牊浼呴敍鍫ュ櫢閺傛澘绱戞慨瀣剁礆
	Clear(ctx context.Context, sessionID string) error
	// ListByUser 閼惧嘲褰囬悽銊﹀煕閻ㄥ嫪绱扮拠婵嗗灙鐞涱煉绱橫ySQL 閺屻儴顕楅敍?
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]Session, int, error)
}

type VectorRepo interface {
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]KnowledgeRef, error)
	Insert(ctx context.Context, collection string, id string, vector []float32) error
}

// 娑撳鐪扮紓鎾崇摠閺嬭埖鐎?type SemanticCache interface {
	// L1: Exact Cache閿涘牏绨跨涵顔煎爱闁板稄绱漅edis String閿涘ey = "exact:hash(query)"閿?
	ExactLookup(ctx context.Context, query string) (reply string, hit bool, err error)
	ExactStore(ctx context.Context, query, reply string) error

	// L2: Semantic Cache閿涘牐顕㈡稊澶屾祲娴肩厧瀹抽崠褰掑帳閿涘苯鎮滈柌蹇旑梾缁鳖澁绱濋惄闀愭妧鎼?>= 0.95 閸涙垝鑵戦敍?
	Lookup(ctx context.Context, vector []float32) (reply string, hit bool, err error)
	Store(ctx context.Context, vector []float32, reply string) error

	// L3: RAG Cache閿涘牏鐓＄拠鍡橆梾缁便垻绮ㄩ弸婊呯处鐎涙﹫绱漅edis Hash閿涘ey = "rag:hash(vector)"閿?
	// 缂傛挸鐡?query vector 閳?knowledge refs 閻ㄥ嫭妲х亸鍕剁礉闁灝鍘ら柌宥咁槻濡偓缁?
	// 闁倻鏁ら崷鐑樻珯閿涙艾娲栨径宥堝窛闁插繋绗夌粙鍐茬暰閿涘奔绲鹃惌銉ㄧ槕鎼存挻顥呯槐銏㈢波閺嬫粌褰叉径宥囨暏
	RAGLookup(ctx context.Context, vector []float32) (knowledge []KnowledgeRef, hit bool, err error)
	RAGStore(ctx context.Context, vector []float32, knowledge []KnowledgeRef) error
}


