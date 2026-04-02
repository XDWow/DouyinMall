//go:build legacy_agent

package domain

type ChatReq struct {
	SessionID string
	UserID    int64
	Message   string
}

// Confidence 鏄唴閮ㄦ寚鏍囷紙褰卞搷鍥炲鏂囨湰鎺緸鍜岃嚜鍔ㄥ崌绾у喅绛栵級锛屼笉瀵瑰鏆撮湶锛涚敱 Prometheus 鐩戞帶銆?
type ChatResp struct {
	Reply              string
	Intent             IntentType
	Knowledge          []KnowledgeRef
	SuggestedQuestions []string        // 涓诲姩鎺ㄨ崘鐨勫叧鑱旈棶棰橈紙2~3 涓級
	HandoffSummary     *HandoffSummary // 杞汉宸ヤ氦鎺ユ憳瑕侊紙浠呰浆浜哄伐鏃堕檮甯︼級
	ToolExecs          []ToolExec      // 鏈疆瀵硅瘽鎵ц鐨勫伐鍏疯皟鐢紙鐢ㄤ簬鍓嶇灞曠ず / 瀹¤鏃ュ織锛?
}

// 涓€娆″伐鍏疯皟鐢ㄧ殑璁板綍
type ToolExec struct {
	Name      string
	Arguments string
	Result    string
	Elapsed   int64 // 鑰楁椂锛堟绉掞級锛岀敤浜庤褰?
}

// AI 鈫?浜哄伐瀹㈡湇浜ゆ帴鎽樿
type HandoffSummary struct {
	CoreIssue        string            // 涓€鍙ヨ瘽姒傛嫭鐢ㄦ埛鏍稿績璇夋眰
	AIActions        []string          // AI 宸茬粡鍋氫簡浠€涔?
	EscalationReason string            // 涓轰粈涔堥渶瑕佽浆浜哄伐
	UserEmotion      string            // neutral / mild_frustration / angry / urgent
	Entities         map[string]string // 鍏抽敭瀹炰綋锛堣鍗曞彿銆佸晢鍝佸悕绛夛級
}

// 鎰忓浘璇嗗埆 + Query 鏀瑰啓缁撴灉锛岄兘鏄熀浜庡綋鍓嶅璇濆拰涓婁笅鏂囷紝涓€璧风敓鎴愬挴
type IntentResult struct {
	Type           IntentType
	Confidence     float32           `json:"confidence"`
	RewrittenQuery string            `json:"rewritten_query"`
	Entities       map[string]string `json:"entities"`
}

// LLM 鐢熸垚闃舵鐨勭粨鏋勫寲杈撳嚭
// 鐢?parseReply 浠?LLM 鍘熷鏂囨湰涓В鏋愬緱鍒帮紝鐢?finalize 娑堣垂涓烘渶缁?ChatResp
// Confidence 鍜?Emotion 鏄笟鍔″喅绛栦緷鎹紙鑷姩杞汉宸ュ垽鏂級
type GenerationResult struct {
	Reply      string
	Confidence float32
	Emotion    string   // neutral / mild_frustration / angry / urgent
	Suggested  []string // 鎺ㄨ崘杩介棶锛?~3 涓級
	MetaSource string   // inline / eval / default
	TokensUsed int
	ToolExecs  []ToolExec // 鏈鐢熸垚涓墽琛岀殑宸ュ叿璋冪敤锛堟棤宸ュ叿璋冪敤鏃朵负绌猴級
}

// ==================== 娴佸紡杈撳嚭 ====================

// 娴佸紡鍒嗙墖绫诲瀷
type ChunkType int

const (
	ChunkStageUpdate ChunkType = iota // Pipeline 闃舵鐘舵€佹帹閫?
	ChunkTextDelta                    // 鍥炲鏂囨湰澧為噺锛堥€愬瓧/閫愬彞锛?
	ChunkDone                         // 缁撴潫鏍囪
)

// 娴佸紡杈撳嚭鍗曚釜鍒嗙墖
type StreamChunk struct {
	Type  ChunkType
	Text  string    // TextDelta 鏃剁殑鏂囨湰澧為噺
	Stage string    // StageUpdate 鏃剁殑闃舵鍚嶇О
	Final *ChatResp // Done 鏃舵惡甯﹀畬鏁村搷搴旓紙鍚?handoff/suggested_questions锛?
}
