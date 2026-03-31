//go:build legacy_agent

package domain

type ChatReq struct {
	SessionID string
	UserID    int64
	Message   string
}

// Confidence 是内部指标（影响回复文本措辞和自动升级决策），不对外暴露；由 Prometheus 监控。
type ChatResp struct {
	Reply              string
	Intent             IntentType
	Knowledge          []KnowledgeRef
	SuggestedQuestions []string        // 主动推荐的关联问题（2~3 个）
	HandoffSummary     *HandoffSummary // 转人工交接摘要（仅转人工时附带）
	ToolExecs          []ToolExec      // 本轮对话执行的工具调用（用于前端展示 / 审计日志）
}

// 一次工具调用的记录
type ToolExec struct {
	Name      string
	Arguments string
	Result    string
	Elapsed   int64 // 耗时（毫秒），用于记录
}

// AI → 人工客服交接摘要
type HandoffSummary struct {
	CoreIssue        string            // 一句话概括用户核心诉求
	AIActions        []string          // AI 已经做了什么
	EscalationReason string            // 为什么需要转人工
	UserEmotion      string            // neutral / mild_frustration / angry / urgent
	Entities         map[string]string // 关键实体（订单号、商品名等）
}

// 意图识别 + Query 改写结果，都是基于当前对话和上下文，一起生成咯
type IntentResult struct {
	Type           IntentType
	Confidence     float32           `json:"confidence"`
	RewrittenQuery string            `json:"rewritten_query"`
	Entities       map[string]string `json:"entities"`
}

// LLM 生成阶段的结构化输出
// 由 parseReply 从 LLM 原始文本中解析得到，由 finalize 消费为最终 ChatResp
// Confidence 和 Emotion 是业务决策依据（自动转人工判断）
type GenerationResult struct {
	Reply      string
	Confidence float32
	Emotion    string   // neutral / mild_frustration / angry / urgent
	Suggested  []string // 推荐追问（2~3 个）
	MetaSource string   // inline / eval / default
	TokensUsed int
	ToolExecs  []ToolExec // 本次生成中执行的工具调用（无工具调用时为空）
}

// ==================== 流式输出 ====================

// 流式分片类型
type ChunkType int

const (
	ChunkStageUpdate ChunkType = iota // Pipeline 阶段状态推送
	ChunkTextDelta                    // 回复文本增量（逐字/逐句）
	ChunkDone                         // 结束标记
)

// 流式输出单个分片
type StreamChunk struct {
	Type  ChunkType
	Text  string    // TextDelta 时的文本增量
	Stage string    // StageUpdate 时的阶段名称
	Final *ChatResp // Done 时携带完整响应（含 handoff/suggested_questions）
}
