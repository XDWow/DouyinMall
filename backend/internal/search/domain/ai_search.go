package domain

// ==================== AI 鎼滅储棰嗗煙妯″瀷 ====================

// QueryIntent LLM 瑙ｆ瀽鍚庣殑鐢ㄦ埛鏌ヨ鎰忓浘
type QueryIntent struct {
	RewrittenQuery string
	Categories     []string
	MinPrice       int64
	MaxPrice       int64
	SortBy         string
	Intent         string
	NeedRAG        bool
}

// RecallResult 鍗曡矾鍙洖缁撴灉锛堣瀺鍚堟帓搴忓墠鐨勪腑闂寸粨鏋滐級
type RecallResult struct {
	ProductID  int64
	Score      float32
	Source     RecallSource
	SalesCount int64 // 涓氬姟淇″彿锛氱敤浜庢帓搴忓姞鏉?
}

type RecallSource string

const (
	RecallKeyword RecallSource = "keyword"
	RecallVector  RecallSource = "vector"
)

// PipelineMetrics 绠＄嚎鍚勯樁娈佃€楁椂锛堟绉掞級锛屼緵鍓嶇/鐩戞帶浣跨敤
type PipelineMetrics struct {
	QueryUnderstandingMs int64
	KeywordRecallMs      int64
	VectorRecallMs       int64
	RankingMs            int64
	FetchMs              int64
	RAGMs                int64
	TotalMs              int64
	KeywordRecallCount   int32 // 鍏抽敭璇嶅彫鍥炴暟閲?
	VectorRecallCount    int32 // 鍚戦噺鍙洖鏁伴噺
}

// AISearchProductsReq AI 鎼滅储璇锋眰
type AISearchProductsReq struct {
	Query           string
	Page            int64
	PageSize        int64
	EnableRAG       bool
	EnableHighlight bool
}

// AISearchProductsResp AI 鎼滅储鍝嶅簲
type AISearchProductsResp struct {
	Products []ProductSearchResult
	Total    int64
	Page     int64
	PageSize int64

	QueryIntent *QueryIntent
	RAGSummary  string
	Metrics     *PipelineMetrics // 绠＄嚎鍙娴嬫€?
}


