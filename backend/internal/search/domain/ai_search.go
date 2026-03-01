package domain

// ==================== AI 搜索领域模型 ====================

// QueryIntent LLM 解析后的用户查询意图
type QueryIntent struct {
	RewrittenQuery string
	Categories     []string
	MinPrice       int64
	MaxPrice       int64
	SortBy         string
	Intent         string
	NeedRAG        bool
}

// RecallResult 单路召回结果（融合排序前的中间结果）
type RecallResult struct {
	ProductID  int64
	Score      float32
	Source     RecallSource
	SalesCount int64 // 业务信号：用于排序加权
}

type RecallSource string

const (
	RecallKeyword RecallSource = "keyword"
	RecallVector  RecallSource = "vector"
)

// PipelineMetrics 管线各阶段耗时（毫秒），供前端/监控使用
type PipelineMetrics struct {
	QueryUnderstandingMs int64
	KeywordRecallMs      int64
	VectorRecallMs       int64
	RankingMs            int64
	FetchMs              int64
	RAGMs                int64
	TotalMs              int64
	KeywordRecallCount   int32 // 关键词召回数量
	VectorRecallCount    int32 // 向量召回数量
}

// AISearchProductsReq AI 搜索请求
type AISearchProductsReq struct {
	Query           string
	Page            int64
	PageSize        int64
	EnableRAG       bool
	EnableHighlight bool
}

// AISearchProductsResp AI 搜索响应
type AISearchProductsResp struct {
	Products []ProductSearchResult
	Total    int64
	Page     int64
	PageSize int64

	QueryIntent *QueryIntent
	RAGSummary  string
	Metrics     *PipelineMetrics // 管线可观测性
}
