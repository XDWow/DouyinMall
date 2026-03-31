package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	infraai "github.com/XDWow/DouyinMall/backend/internal/agent/infra/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/mcp"
)

// AIService 封装所有 LLM / Embedding / 知识库相关的 AI 能力，为
// 无状态：不持有 Session，不做持久化，不做业务决策（转人工、限频等）
// ChatUseCase 作为编排层调用 AIService 的方法，传入上下文数据，拿回结构化结果
type AIService struct {
	llm           infraai.CSLLMClient // 使用 infra/ai 的接口
	embedder      ai.Embedder
	knowledgeRepo domain.VectorRepo
	cache         domain.SemanticCache
	reranker      ai.Reranker // cross-encoder 重排模型，nil 时降级为 top-N 截断
	// 提供 tool call 能力
	mcpClient mcp.MCPClient
	toolDefs  []ai.ToolDef

	metrics *PipelineMetrics
	logger  logger.LoggerV1
}

func NewAIService(
	llm infraai.CSLLMClient, // 使用 infra/ai 的接口
	embedder ai.Embedder,
	reranker ai.Reranker,
	knowledgeRepo domain.VectorRepo,
	cache domain.SemanticCache,
	mcpClient mcp.MCPClient,
	metrics *PipelineMetrics,
	logger logger.LoggerV1,
) *AIService {
	s := &AIService{
		llm:           llm,
		embedder:      embedder,
		reranker:      reranker,
		knowledgeRepo: knowledgeRepo,
		cache:         cache,
		mcpClient:     mcpClient,
		metrics:       metrics,
		logger:        logger,
	}
	// 工具 schema 分两套：
	// - MCP Server schema：面向业务服务，含真实 ID（product_id/user_id），由 MCP Server 维护
	// - LLM-facing schema：面向模型，用语义参数（product_ref/source），屏蔽所有 ID，防止 LLM 幻觉
	// agent 作为 mcp client 在 executeToolCall 中通过 resolveToolArgs 完成两套 schema 之间的转换（注入真实 ID）
	if mcpClient != nil && len(mcpClient.Tools()) > 0 {
		s.toolDefs = llmFacingToolDefs()
	}
	return s
}

type EmbedResult struct {
	Vectors [][]float32
	Err     error
}

// 对文本做向量化
func (s *AIService) Embed(ctx context.Context, text string) EmbedResult {
	vectors, err := s.embedder.Embed(ctx, []string{text})
	return EmbedResult{Vectors: vectors, Err: err}
}

// L1: Exact Cache（精确匹配，Redis String，key = "exact:hash(query)"）
func (s *AIService) ExactCacheLookup(ctx context.Context, query string) (reply string, hit bool) {
	reply, hit, err := s.cache.ExactLookup(ctx, query)
	if err != nil {
		s.logger.Warn("精确缓存查询异常", logger.Error(err))
		return "", false
	}
	return reply, hit
}

func (s *AIService) ExactCacheStore(ctx context.Context, query, reply string) {
	if err := s.cache.ExactStore(ctx, query, reply); err != nil {
		s.logger.Warn("精确缓存写入失败", logger.Error(err))
	}
}

// L2: Semantic Cache（语义相似度匹配，向量检索，相似度 ≥ 0.95 命中）
func (s *AIService) SemanticCacheLookup(ctx context.Context, vector []float32) (reply string, hit bool) {
	reply, hit, err := s.cache.Lookup(ctx, vector)
	if err != nil {
		s.logger.Warn("语义缓存查询异常", logger.Error(err))
		return "", false
	}
	return reply, hit
}

func (s *AIService) SemanticCacheStore(ctx context.Context, vector []float32, reply string) {
	if err := s.cache.Store(ctx, vector, reply); err != nil {
		s.logger.Warn("语义缓存写入失败", logger.Error(err))
	}
}

// L3: RAG Cache（知识检索结果缓存，Redis Hash）
func (s *AIService) RAGCacheLookup(ctx context.Context, vector []float32) ([]domain.KnowledgeRef, bool) {
	knowledge, hit, err := s.cache.RAGLookup(ctx, vector)
	if err != nil {
		s.logger.Warn("RAG缓存查询异常", logger.Error(err))
		return nil, false
	}
	return knowledge, hit
}

func (s *AIService) RAGCacheStore(ctx context.Context, vector []float32, knowledge []domain.KnowledgeRef) {
	if err := s.cache.RAGStore(ctx, vector, knowledge); err != nil {
		s.logger.Warn("RAG缓存写入失败", logger.Error(err))
	}
}

// 意图识别 + Query 改写，合并成一次调用
// history: 最近几轮对话（由编排层从 Session 中截取）

func (s *AIService) RecognizeIntent(ctx context.Context, message string, history []domain.Message) (*domain.IntentResult, error) {
	var historyStr string
	if len(history) > 0 {
		var sb strings.Builder
		for _, m := range history {
			fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
		}
		historyStr = sb.String()
	} else {
		historyStr = "（无历史）"
	}

	resp, err := s.llm.ChatCompletion(ctx, infraai.ChatRequest{
		Messages: []ai.Message{{Role: "system", Content: fmt.Sprintf(intentPrompt, historyStr, message)}},
	})
	if err != nil {
		return nil, err
	}

	content := cleanJSON(resp.Choices[0].Message.Content)
	result := &domain.IntentResult{
		Type:           domain.IntentUnknown,
		RewrittenQuery: message,
	}
	var aux struct {
		Intent string `json:"intent"`
		*domain.IntentResult
	}
	aux.IntentResult = result
	if err := json.Unmarshal([]byte(content), &aux); err != nil {
		s.logger.Warn("解析意图结果失败", logger.Error(err), logger.String("raw", content))
		return result, nil
	}
	result.Type = mapIntent(aux.Intent)
	return result, nil
}

const vectorCandidateCount = 20 // 向量召回初始候选条数，rerankByLLM 再精选 topN 条

// 两阶段 RAG：向量召回(粗) + cross-encoder 重排（精）
func (s *AIService) Retrieve(ctx context.Context, query string, vector []float32, topN int) []domain.KnowledgeRef {
	candidates, err := s.knowledgeRepo.Search(ctx, domain.CollectionKnowledge, vector, vectorCandidateCount)
	if err != nil || len(candidates) == 0 {
		return nil
	}
	return s.rerank(ctx, query, candidates, topN)
}

// 用 cross-encoder 模型对候选文档按相关性重排，取 topN
// reranker 为 nil 时降级为直接截断（向量召回顺序即为结果）
func (s *AIService) rerank(ctx context.Context, query string, candidates []domain.KnowledgeRef, topN int) []domain.KnowledgeRef {
	if len(candidates) <= topN {
		return candidates
	}
	if s.reranker == nil {
		s.logger.Warn("reranker 未配置，降级返回前 N 条")
		return candidates[:topN]
	}

	docs := make([]string, len(candidates))
	for i, c := range candidates {
		docs[i] = fmt.Sprintf("[%s] %s: %s", c.Category, c.Title, c.Content)
	}

	scores, err := s.reranker.Rerank(ctx, query, docs)
	if err != nil {
		s.logger.Warn("rerank 失败，降级返回前 N 条", logger.Error(err))
		return candidates[:topN]
	}

	// 按分数排序（降序），取 topN
	type scored struct {
		ref   domain.KnowledgeRef
		score float32
	}
	scored_ := make([]scored, len(candidates))
	for i, c := range candidates {
		scored_[i] = scored{ref: c, score: scores[i]}
	}
	sort.Slice(scored_, func(i, j int) bool {
		return scored_[i].score > scored_[j].score
	})

	result := make([]domain.KnowledgeRef, topN)
	for i := range result {
		result[i] = scored_[i].ref
	}
	return result
}

type GenerateReq struct {
	UserID    int64
	Message   string
	History   []domain.Message // 滑动窗口内的最近消息
	Knowledge []domain.KnowledgeRef
	State     *domain.EntityMemory // 对话状态（工具模式下注入 prompt，nil=不注入）
}

// Generate 同步生成回复
// 若 toolDefs 不为空，自动走工具调用循环；否则直接单轮生成
func (s *AIService) Generate(ctx context.Context, req GenerateReq) *domain.GenerationResult {
	messages := s.buildMessages(req)

	if len(s.toolDefs) == 0 {
		resp, err := s.llm.ChatCompletion(ctx, infraai.ChatRequest{
			Messages: messages,
		})
		if err != nil {
			s.logger.Warn("Generate LLM 调用失败（所有节点均失败，已走模板兜底）", logger.Error(err))
		}
		if resp == nil || len(resp.Choices) == 0 {
			s.logger.Error("Generate: LLM 返回 nil resp，无法解析回复")
			return &domain.GenerationResult{Reply: "抱歉，系统繁忙，请稍后重试。", Confidence: 0.3, Emotion: "neutral"}
		}
		gen := parseReply(resp.Choices[0].Message.Content)
		gen.TokensUsed = resp.TokensUsed
		s.logger.Info("Generate 完成",
			logger.String("reply_prefix", gen.Reply[:min(len(gen.Reply), 80)]),
			logger.Int("tokens", resp.TokensUsed))
		return gen
	}

	toolExecs := make([]domain.ToolExec, 0, 4)
	// for 循环实现链式调用，并指定最大长度
	for round := 0; round < maxToolRounds; round++ {
		resp, err := s.llm.ChatCompletion(ctx, infraai.ChatRequest{
			Messages: messages,
			Tools:    s.toolDefs,
		})
		if err != nil {
			s.logger.Warn("tool calling LLM 调用失败", logger.Error(err))
			break
		}

		if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
			gen := parseReply(resp.Choices[0].Message.Content)
			gen.TokensUsed = resp.TokensUsed
			gen.ToolExecs = toolExecs
			return gen
		}

		messages = append(messages, ai.Message{
			Role: "assistant", Content: resp.Choices[0].Message.Content, ToolCalls: resp.Choices[0].Message.ToolCalls,
		})
		// llm 返回要调用的 tool，我这后端服务 agent 作为 mcp client 向 mcp server 发请求
		for _, tc := range resp.Choices[0].Message.ToolCalls {
			exec := s.executeToolCall(ctx, tc, req.State, req.UserID)
			toolExecs = append(toolExecs, exec)
			messages = append(messages, ai.Message{
				Role: "tool", Content: exec.Result, ToolCallID: tc.ID,
			})
		}
	}

	// 到这里说明超过最大轮数，最终一次不带 tools 的调用，基于尽量的工具调用结果，逼 llm 输出最终文字回复，也就是强行结束
	resp, _ := s.llm.ChatCompletion(ctx, infraai.ChatRequest{
		Messages: messages,
	})
	gen := parseReply(resp.Choices[0].Message.Content)
	gen.TokensUsed = resp.TokensUsed
	gen.ToolExecs = toolExecs
	return gen
}

// 流式生成回复
// 无论是否有工具，第一次都走流式调用：
//   - finish_reason=stop       → 文字已经流式推给用户，直接结束
//   - finish_reason=tool_calls → 同步执行工具，再发起流式调用输出最终回复
func (s *AIService) GenerateStream(ctx context.Context, req GenerateReq, send func(domain.StreamChunk)) *domain.GenerationResult {
	messages := s.buildMessages(req)
	toolExecs := make([]domain.ToolExec, 0, 4)

	for round := 0; round <= maxToolRounds; round++ {
		chatReq := infraai.ChatRequest{
			Messages: messages,
			Tools:    s.toolDefs,
		}

		// 流式调用，消费 token 并检测 finish_reason
		if streamCh, err := s.llm.ChatCompletionStream(ctx, chatReq); err == nil {
			// 读 streamCh，增量文本就用 send 写回 channel，如果遇到工具调用需求/对话结束，会返回
			toolCalls, content := s.drainTokensWithTools(streamCh, send)

			if len(toolCalls) == 0 { // finish_reason=stop：对话结束
				// 流式推送时只推 ===META=== 前面的文字给用户，META 后面的 JSON 不推，最后一次返回的是后面的 置信度，情绪，建议的问题等
				gen := parseReply(content)
				gen.ToolExecs = toolExecs // 调用的工具也收集起来，用于上层 session 记录
				return gen
			}

			// finish_reason=tool_calls：同步执行工具，继续下一轮
			messages = append(messages, ai.Message{
				Role: "assistant", ToolCalls: toolCalls,
			})
			send(domain.StreamChunk{Type: domain.ChunkStageUpdate, Stage: "tool_calling"})
			for _, tc := range toolCalls {
				exec := s.executeToolCall(ctx, tc, req.State, req.UserID)
				toolExecs = append(toolExecs, exec)
				messages = append(messages, ai.Message{
					Role: "tool", Content: exec.Result, ToolCallID: tc.ID,
				})
			}
			continue
		} else {
			s.logger.Warn("流式调用失败，降级同步", logger.Error(err))
		}

		// 降级同步路径（流式调用失败时）
		s.metrics.IncFallbackSync()
		resp, err := s.llm.ChatCompletion(ctx, chatReq)
		if err != nil {
			s.logger.Warn("同步 LLM 调用失败", logger.Error(err))
			break
		}
		if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
			gen := parseReply(resp.Choices[0].Message.Content)
			gen.TokensUsed = resp.TokensUsed
			gen.ToolExecs = toolExecs
			send(domain.StreamChunk{Type: domain.ChunkTextDelta, Text: gen.Reply})
			return gen
		}
		messages = append(messages, ai.Message{
			Role: "assistant", Content: resp.Choices[0].Message.Content, ToolCalls: resp.Choices[0].Message.ToolCalls,
		})
		send(domain.StreamChunk{Type: domain.ChunkStageUpdate, Stage: "tool_calling"})
		for _, tc := range resp.Choices[0].Message.ToolCalls {
			exec := s.executeToolCall(ctx, tc, req.State, req.UserID)
			toolExecs = append(toolExecs, exec)
			messages = append(messages, ai.Message{
				Role: "tool", Content: exec.Result, ToolCallID: tc.ID,
			})
		}
	}

	// 超过最大轮数兜底
	gen := s.streamFinalReply(ctx, messages, send)
	gen.ToolExecs = toolExecs
	return gen
}

// 调用 llm 生成交接摘要
func (s *AIService) BuildHandoff(ctx context.Context, history []domain.Message) *domain.HandoffSummary {
	var sb strings.Builder
	for _, m := range history {
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
	}

	resp, err := s.llm.ChatCompletion(ctx, infraai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: fmt.Sprintf(handoffPrompt, sb.String())}},
	})
	if err != nil {
		s.logger.Warn("Handoff Summary 生成失败，降级", logger.Error(err))
		return defaultHandoff(history)
	}

	content := cleanJSON(resp.Choices[0].Message.Content)
	var parsed struct {
		CoreIssue        string            `json:"core_issue"`
		AIActions        []string          `json:"ai_actions"`
		EscalationReason string            `json:"escalation_reason"`
		UserEmotion      string            `json:"user_emotion"`
		Entities         map[string]string `json:"entities"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		s.logger.Warn("解析 Handoff Summary 失败，降级", logger.Error(err), logger.String("raw", content))
		return defaultHandoff(history)
	}

	return &domain.HandoffSummary{
		CoreIssue:        parsed.CoreIssue,
		AIActions:        parsed.AIActions,
		EscalationReason: parsed.EscalationReason,
		UserEmotion:      parsed.UserEmotion,
		Entities:         parsed.Entities,
	}
}

// EnsureMeta 在主回复未成功解析 inline META 时，走一次二次评估兜底，补齐 confidence/emotion。
// 说明：
//   - 主路径：parseReply 从 ===META=== 解析（MetaSource=inline）
//   - 兜底路径：若格式异常/截断/缺失（MetaSource!=inline），再调用一次小任务 prompt 评估
func (s *AIService) EnsureMeta(ctx context.Context, req GenerateReq, gen *domain.GenerationResult) {
	if gen == nil {
		return
	}
	if gen.MetaSource == "inline" {
		return
	}
	reply := strings.TrimSpace(gen.Reply)
	if reply == "" {
		return
	}

	var historyText strings.Builder
	for _, m := range req.History {
		fmt.Fprintf(&historyText, "%s: %s\n", m.Role, m.Content)
	}
	if historyText.Len() == 0 {
		historyText.WriteString("（无历史）")
	}

	resp, err := s.llm.ChatCompletion(ctx, infraai.ChatRequest{
		Messages: []ai.Message{{
			Role: "system",
			Content: fmt.Sprintf(metaEvalPrompt,
				historyText.String(),
				req.Message,
				reply,
			),
		}},
	})
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		s.logger.Warn("二次评估 meta 失败，保持默认值",
			logger.Error(err),
			logger.String("meta_source", gen.MetaSource))
		return
	}

	var parsed struct {
		Confidence         float32  `json:"confidence"`
		Emotion            string   `json:"emotion"`
		SuggestedQuestions []string `json:"suggested_questions"`
	}
	metaText := cleanJSON(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(metaText), &parsed); err != nil {
		s.logger.Warn("二次评估 meta 解析失败，保持默认值",
			logger.Error(err),
			logger.String("raw", metaText))
		return
	}

	if parsed.Confidence < 0 || parsed.Confidence > 1 || !isValidEmotion(parsed.Emotion) {
		s.logger.Warn("二次评估 meta 非法，保持默认值",
			logger.String("confidence", fmt.Sprintf("%.4f", parsed.Confidence)),
			logger.String("emotion", parsed.Emotion))
		return
	}

	gen.Confidence = parsed.Confidence
	gen.Emotion = strings.TrimSpace(parsed.Emotion)
	if len(gen.Suggested) == 0 && len(parsed.SuggestedQuestions) > 0 {
		gen.Suggested = parsed.SuggestedQuestions
	}
	gen.MetaSource = "eval"
}

// =========== 内部方法 =================================

// 拼装 LLM 输入（系统提示 + 摘要 + 知识 + 历史 + 当前输入）
func (s *AIService) buildMessages(req GenerateReq) []ai.Message {
	sysPrompt := systemPrompt
	if len(s.toolDefs) > 0 {
		sysPrompt = toolSystemPrompt
	}
	msgs := []ai.Message{{Role: "system", Content: sysPrompt}}

	// 对话状态上下文（仅工具模式，让 LLM 解析"第一个"/"买这个"等引用）
	if len(s.toolDefs) > 0 {
		if stateCtx := formatStateContext(req.State); stateCtx != "" {
			msgs = append(msgs, ai.Message{Role: "system", Content: stateCtx})
		}
	}

	if len(req.Knowledge) > 0 {
		var sb strings.Builder
		for i, k := range req.Knowledge {
			fmt.Fprintf(&sb, "%d. [%s] %s\n%s\n\n", i+1, k.Category, k.Title, k.Content)
		}
		msgs = append(msgs, ai.Message{Role: "system", Content: "【知识库上下文】\n" + sb.String()})
	}

	for _, m := range req.History {
		msgs = append(msgs, ai.Message{Role: string(m.Role), Content: m.Content})
	}

	msgs = append(msgs, ai.Message{Role: "user", Content: req.Message})
	return msgs
}

const maxToolRounds = 5 // 最多允许的工具调用轮数，防止无限循环

// 消费流式响应，实时推送文本给前端，并返回工具调用列表。
// 返回：(toolCalls, fullContent)
//   - toolCalls 非空 → finish_reason=tool_calls，调用方执行工具后再发起新一轮流式
//   - toolCalls 为空 → finish_reason=stop，文字已全部推出，直接结束
//
// 推送逻辑：每收到一个增量文本立即推送，但保留 len(metaSep)-1 字节的尾部缓冲，
// 防止分隔符跨 chunk 被截断推出给前端。
//
// 注意：infra/ai 层已经完成了工具参数累积，这里只需要处理文本推送和 META 分隔符
func (s *AIService) drainTokensWithTools(
	streamCh <-chan infraai.ChatResponse,
	send func(domain.StreamChunk),
) ([]ai.ToolCall, string) {
	var buf strings.Builder
	pushedLen := 0
	const metaSep = "===META==="

	var toolCalls []ai.ToolCall

	for resp := range streamCh {
		if len(resp.Choices) == 0 {
			continue
		}

		choice := resp.Choices[0]

		// infra/ai 层累积完成后会发送完整的 tool_calls
		if len(choice.Message.ToolCalls) > 0 {
			toolCalls = choice.Message.ToolCalls
			if choice.Message.Content != "" {
				buf.WriteString(choice.Message.Content)
			}
			break
		}

		// 累积增量文本
		delta := choice.Message.Content
		if delta == "" {
			continue
		}

		buf.WriteString(delta)
		full := buf.String()

		// 找到 META 分隔符：推送分隔符之前的内容，之后停推
		if idx := strings.Index(full, metaSep); idx >= 0 {
			if idx > pushedLen {
				send(domain.StreamChunk{Type: domain.ChunkTextDelta, Text: full[pushedLen:idx]})
			}
			pushedLen = len(full)
			continue
		}

		// 尚未找到分隔符：安全推送，保留 len(metaSep)-1 字节防止分隔符被截断
		safeEnd := len(full) - (len(metaSep) - 1)
		if safeEnd > pushedLen {
			send(domain.StreamChunk{Type: domain.ChunkTextDelta, Text: full[pushedLen:safeEnd]})
			pushedLen = safeEnd
		}
	}

	// 推送尾部剩余内容（无 META 分隔符时）
	full := buf.String()
	if pushedLen < len(full) {
		tail := strings.TrimSpace(full[pushedLen:])
		if tail != "" {
			send(domain.StreamChunk{Type: domain.ChunkTextDelta, Text: tail})
		}
	}

	return toolCalls, full
}

// 返回面向模型的工具 schema
// LLM 只表达语义引用，不直接操作数据库 ID。
// 原因：
// 1. ID 对 LLM 没有语义结构，只是相似的数字 token，而 LLM 又是概率生成模型，因此生成精确 ID 不可靠，容易产生幻觉，也就是编造 ID
// 2. 敏感 ID（user_id / order_id）需由服务端注入以保证安全。
// Agent 在调用 MCP 前负责将语义引用解析为真实业务 ID。

// 与 MCP Server schema 的区别：
//   - 用语义参数（product_ref="list_0"/"current"）代替真实 ID（product_id）
//   - 不含 user_id，由 resolveToolArgs 在调用 MCP 前自动注入
//
// LLM 只需描述"意图"，agent 负责把意图翻译成真实业务参数
func llmFacingToolDefs() []ai.ToolDef {
	return []ai.ToolDef{
		{Type: "function", Function: ai.FunctionDef{
			Name:        "search_products",
			Description: "搜索商品，返回匹配列表",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"搜索关键词"},"page":{"type":"integer"},"page_size":{"type":"integer"}},"required":["query"]}`),
		}},
		{Type: "function", Function: ai.FunctionDef{
			Name:        "get_product_detail",
			Description: "查看商品详情。product_ref=\"list_0\"查搜索结果第1个，\"list_1\"第2个，以此类推；\"current\"查当前商品",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"product_ref":{"type":"string","description":"商品引用：list_0/list_1/... 表示搜索结果第N项（0起），current 表示当前商品"}},"required":["product_ref"]}`),
		}},
		{Type: "function", Function: ai.FunctionDef{
			Name:        "add_to_cart",
			Description: "将指定商品加入购物车",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"product_ref":{"type":"string","description":"商品引用：list_0/list_1/... 或 current"},"quantity":{"type":"integer","description":"加购数量，默认1"}},"required":["product_ref"]}`),
		}},
		{Type: "function", Function: ai.FunctionDef{
			Name:        "get_cart",
			Description: "查看购物车内容",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		}},
		{Type: "function", Function: ai.FunctionDef{
			Name:        "create_order",
			Description: "下单。source=product 购买指定商品；source=cart 结算购物车全部商品",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"source":{"type":"string","enum":["product","cart"]},"product_ref":{"type":"string","description":"source=product时必填，list_0/list_1/... 或 current"},"quantity":{"type":"integer","description":"购买数量，source=product时有效，默认1"}},"required":["source"]}`),
		}},
		{Type: "function", Function: ai.FunctionDef{
			Name:        "get_order",
			Description: "查询订单状态。可填用户说的订单号；不填则查最近订单",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"order_id":{"type":"string","description":"订单号，不填则查最近订单"}}}`),
		}},
	}
}

// 将 LLM 输出的商品引用解析为真实 product_id 和商品名
// list_N: 搜索结果第 N 项（0-indexed）；current/"": 当前查看/最近操作商品
// LLM 只需传 product_ref，永远不需要知道真实 ID
func resolveProductRef(ref string, state *domain.EntityMemory) (id, name string) {
	if state == nil {
		return "", ""
	}
	if ref == "current" || ref == "" {
		return state.CurrentProductID, state.CurrentProductName
	}
	if strings.HasPrefix(ref, "list_") {
		idx, err := strconv.Atoi(strings.TrimPrefix(ref, "list_"))
		if err == nil && idx >= 0 && idx < len(state.ProductList) {
			p := state.ProductList[idx]
			return p.ProductID, p.Name
		}
	}
	return "", ""
}

// 在调用 MCP 前，将 LLM 的语义参数补充为完整业务参数
// LLM 只输出 product_ref / quantity / source / query 等语义字段，
// product_id / user_id 等所有 ID 由此函数注入，LLM 从不感知
func resolveToolArgs(toolName, rawArgs string, state *domain.EntityMemory, userID int64) json.RawMessage {
	var args map[string]any
	_ = json.Unmarshal([]byte(rawArgs), &args)
	if args == nil {
		args = make(map[string]any)
	}

	// user_id 注入：所有需要用户身份的工具
	switch toolName {
	case "add_to_cart", "get_cart", "create_order", "get_order":
		args["user_id"] = fmt.Sprintf("%d", userID)
	}

	if state == nil {
		data, _ := json.Marshal(args)
		return data
	}

	switch toolName {
	case "get_product_detail":
		ref, _ := args["product_ref"].(string)
		productID, _ := resolveProductRef(ref, state)
		args["product_id"] = productID
		delete(args, "product_ref")

	case "add_to_cart":
		ref, _ := args["product_ref"].(string)
		productID, _ := resolveProductRef(ref, state)
		args["product_id"] = productID
		delete(args, "product_ref")
		if _, ok := args["quantity"]; !ok {
			args["quantity"] = 1
		}

	case "create_order":
		if src, _ := args["source"].(string); src == "product" {
			ref, _ := args["product_ref"].(string)
			productID, _ := resolveProductRef(ref, state)
			args["product_id"] = productID
		}
		delete(args, "product_ref")

	case "get_order": // 用户可以指定订单号，如果没有指定，那就默认最近的订单ID
		if v, _ := args["order_id"].(string); v == "" && state.LastOrderID != "" {
			args["order_id"] = state.LastOrderID
		}
	}

	data, _ := json.Marshal(args)
	return data
}

// 通过 MCP 执行单次工具调用
// 在调用前由 resolveToolArgs 补全 ID，MCP Server 接收完整参数，LLM 从不感知 ID
func (s *AIService) executeToolCall(ctx context.Context, tc ai.ToolCall, state *domain.EntityMemory, userID int64) domain.ToolExec {
	start := time.Now()
	exec := domain.ToolExec{
		Name:      tc.Function.Name,
		Arguments: tc.Function.Arguments,
	}

	resolvedArgs := resolveToolArgs(tc.Function.Name, tc.Function.Arguments, state, userID)
	result, err := s.mcpClient.CallTool(ctx, tc.Function.Name, resolvedArgs)
	exec.Elapsed = time.Since(start).Milliseconds()

	if err != nil {
		s.logger.Warn("MCP 工具调用失败",
			logger.String("tool", tc.Function.Name),
			logger.Error(err))
		exec.Result = fmt.Sprintf("工具调用失败: %s", err.Error())
		return exec
	}

	var sb strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	exec.Result = sb.String()
	return exec
}

// 流式的工具调用限额兜底：超过最大工具轮数后，强制流式输出最终回复（不再处理工具调用）
func (s *AIService) streamFinalReply(ctx context.Context, messages []ai.Message, send func(domain.StreamChunk)) *domain.GenerationResult {
	chatReq := infraai.ChatRequest{Messages: messages}

	if streamCh, err := s.llm.ChatCompletionStream(ctx, chatReq); err == nil {
		_, content := s.drainTokensWithTools(streamCh, send)
		return parseReply(content)
	} else {
		s.logger.Warn("最终回复流式生成失败，降级同步", logger.Error(err))
	}

	resp, _ := s.llm.ChatCompletion(ctx, chatReq)
	if resp == nil || len(resp.Choices) == 0 {
		return &domain.GenerationResult{Reply: "抱歉，系统繁忙，请稍后重试。", Confidence: 0.3, Emotion: "neutral"}
	}
	gen := parseReply(resp.Choices[0].Message.Content)
	gen.TokensUsed = resp.TokensUsed
	send(domain.StreamChunk{Type: domain.ChunkTextDelta, Text: gen.Reply})
	return gen
}
