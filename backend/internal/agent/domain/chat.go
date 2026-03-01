package domain

// ChatReq 对话请求（usecase 入参）
type ChatReq struct {
	SessionID string
	UserID    int64
	Message   string
	Channel   string
}

// ChatResp 对话响应（usecase 出参）
type ChatResp struct {
	Reply      string
	Intent     IntentType
	Confidence float32
	Knowledge  []KnowledgeRef
	Debug      *PipelineDebug
}

// IntentResult 意图识别 + Query 改写结果
type IntentResult struct {
	Type           IntentType
	Confidence     float32
	RewrittenQuery string            // 改写后的查询（用于 RAG embedding）
	Entities       map[string]string // 提取的实体（如 order_id, product_name）
}

// PipelineDebug 四阶段 Pipeline 各环节耗时
type PipelineDebug struct {
	IntentMs       int64
	EmbedMs        int64
	VectorMs       int64
	RerankMs       int64
	GenerateMs     int64
	ToolMs         int64
	TotalMs        int64
	KnowledgeHits  int32
	CacheHit       bool
	RewrittenQuery string
}
