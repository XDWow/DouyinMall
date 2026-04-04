//go:build legacy_agent

package domain

type ChatReq struct {
	SessionID string
	UserID    int64
	Message   string
}

// Confidence 閺勵垰鍞撮柈銊﹀瘹閺嶅浄绱欒ぐ鍗炴惙閸ョ偛顦查弬鍥ㄦ拱閹侯亣绶搁崪宀冨殰閸斻劌宕岀痪褍鍠呯粵鏍电礆閿涘奔绗夌€电懓顦婚弳鎾苟閿涙稓鏁?Prometheus 閻╂垶甯堕妴?
type ChatResp struct {
	Reply              string
	Intent             IntentType
	Knowledge          []KnowledgeRef
	SuggestedQuestions []string        // 娑撹濮╅幒銊ㄥ礃閻ㄥ嫬鍙ч懕鏃堟６妫版﹫绱?~3 娑擃亷绱?	HandoffSummary     *HandoffSummary // 鏉烆兛姹夊銉ゆ唉閹恒儲鎲崇憰渚婄礄娴犲懓娴嗘禍鍝勪紣閺冨爼妾敮锔肩礆
	ToolExecs          []ToolExec      // 閺堫剝鐤嗙€电鐦介幍褑顢戦惃鍕紣閸忕柉鐨熼悽顭掔礄閻劋绨崜宥囶伂鐏炴洜銇?/ 鐎孤ゎ吀閺冦儱绻旈敍?
}

// 娑撯偓濞嗏€充紣閸忕柉鐨熼悽銊ф畱鐠佹澘缍?type ToolExec struct {
	Name      string
	Arguments string
	Result    string
	Elapsed   int64 // 閼版妞傞敍鍫燁嚑缁夋帪绱氶敍宀€鏁ゆ禍搴ゎ唶瑜?
}

// AI 閳?娴滃搫浼愮€广垺婀囨禍銈嗗复閹芥顩?type HandoffSummary struct {
	CoreIssue        string            // 娑撯偓閸欍儴鐦藉鍌涘閻劍鍩涢弽绋跨妇鐠囧鐪?	AIActions        []string          // AI 瀹歌尙绮￠崑姘啊娴犫偓娑?
	EscalationReason string            // 娑撹桨绮堟稊鍫ユ付鐟曚浇娴嗘禍鍝勪紣
	UserEmotion      string            // neutral / mild_frustration / angry / urgent
	Entities         map[string]string // 閸忔娊鏁€圭偘缍嬮敍鍫ｎ吂閸楁洖褰块妴浣告櫌閸濅礁鎮曠粵澶涚礆
}

// 閹板繐娴樼拠鍡楀焼 + Query 閺€鐟板晸缂佹挻鐏夐敍宀勫厴閺勵垰鐔€娴滃骸缍嬮崜宥咁嚠鐠囨繂鎷版稉濠佺瑓閺傚浄绱濇稉鈧挧椋庢晸閹存劕鎸?type IntentResult struct {
	Type           IntentType
	Confidence     float32           `json:"confidence"`
	RewrittenQuery string            `json:"rewritten_query"`
	Entities       map[string]string `json:"entities"`
}

// LLM 閻㈢喐鍨氶梼鑸殿唽閻ㄥ嫮绮ㄩ弸鍕鏉堟挸鍤?// 閻?parseReply 娴?LLM 閸樼喎顫愰弬鍥ㄦ拱娑擃叀袙閺嬫劕绶遍崚甯礉閻?finalize 濞戝牐鍨傛稉鐑樻付缂?ChatResp
// Confidence 閸?Emotion 閺勵垯绗熼崝鈥冲枀缁涙牔绶烽幑顕嗙礄閼奉亜濮╂潪顑挎眽瀹搞儱鍨介弬顓ㄧ礆
type GenerationResult struct {
	Reply      string
	Confidence float32
	Emotion    string   // neutral / mild_frustration / angry / urgent
	Suggested  []string // 閹恒劏宕樻潻浠嬫６閿?~3 娑擃亷绱?	MetaSource string   // inline / eval / default
	TokensUsed int
	ToolExecs  []ToolExec // 閺堫剚顐奸悽鐔稿灇娑擃厽澧界悰宀€娈戝銉ュ徔鐠嬪啰鏁ら敍鍫熸￥瀹搞儱鍙跨拫鍐暏閺冩湹璐熺粚鐚寸礆
}

// ==================== 濞翠礁绱℃潏鎾冲毉 ====================

// 濞翠礁绱￠崚鍡欏缁鐎?type ChunkType int

const (
	ChunkStageUpdate ChunkType = iota // Pipeline 闂冭埖顔岄悩鑸碘偓浣瑰腹闁?
	ChunkTextDelta                    // 閸ョ偛顦查弬鍥ㄦ拱婢х偤鍣洪敍鍫モ偓鎰摟/闁劕褰為敍?
	ChunkDone                         // 缂佹挻娼弽鍥唶
)

// 濞翠礁绱℃潏鎾冲毉閸楁洑閲滈崚鍡欏
type StreamChunk struct {
	Type  ChunkType
	Text  string    // TextDelta 閺冨墎娈戦弬鍥ㄦ拱婢х偤鍣?	Stage string    // StageUpdate 閺冨墎娈戦梼鑸殿唽閸氬秶袨
	Final *ChatResp // Done 閺冭埖鎯＄敮锕€鐣弫鏉戞惙鎼存棑绱欓崥?handoff/suggested_questions閿?
}


