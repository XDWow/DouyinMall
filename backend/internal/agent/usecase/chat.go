//go:build legacy_agent

package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
)

const (
	maxWindowSize         = 10
	autoEscalateThreshold = 3
	confidenceHigh        = 0.8
	confidenceLow         = 0.5
	maxMessageLen         = 2000

	transferReply  = "正在为您转接人工客服，请稍候..."
	humanWaitReply = "您已连接人工客服，消息已送达，请稍候回复。"

	systemLimitKey  = "agent:system:limit"
	userLimitKeyFmt = "agent:rate:%d" // 用户维度限流 key，%d 为 userID
)

// ChatInput Handler 层传入的原始请求参数，由 ChatUseCase 负责校验后转为 domain.ChatReq。
type ChatInput struct {
	SessionID string
	UserID    int64
	Message   string
}

func (in ChatInput) validate() error {
	if in.SessionID == "" {
		return errors.New("session_id 不能为空")
	}
	if in.UserID <= 0 {
		return errors.New("user_id 必须大于 0")
	}
	if in.Message == "" {
		return errors.New("消息内容不能为空")
	}
	if len([]rune(in.Message)) > maxMessageLen {
		return errors.New("消息内容超出长度限制")
	}
	return nil
}

func (in ChatInput) toDomain() *domain.ChatReq {
	return &domain.ChatReq{
		SessionID: in.SessionID,
		UserID:    in.UserID,
		Message:   in.Message,
	}
}

// 三层职责
//
//	session（SessionRepo + domain）：会话生命周期、消息持久化、实体记忆存储，不知道 LLM 存在
//	AIService：与 LLM/Embedding/MCP 交互，工具调用循环，流式推送，不知道有 HTTP 请求和会话
//	ChatUseCase：编排层，调 session 加载会话 → 调 AIService 生成回复 → 调 session 持久化，把两边粘合起来，处理限频/转人工/缓存等业务规则
type ChatUseCase struct {
	ai            *AIService
	sessionRepo   domain.SessionRepo
	systemLimiter ratelimit.Limiter // 系统总限流（Redis 滑动窗口，多实例共享）
	userLimiter   ratelimit.Limiter // 用户维度限流（Redis 滑动窗口，key = agent:rate:<userID>）
	metrics       *PipelineMetrics
	logger        logger.LoggerV1
}

func NewChatUseCase(
	ai *AIService,
	sessionRepo domain.SessionRepo,
	systemLimiter ratelimit.Limiter, // 系统总限流，优先于用户限流
	userLimiter ratelimit.Limiter, // 用户维度限流，key 在调用时注入
	metrics *PipelineMetrics,
	logger logger.LoggerV1,
) *ChatUseCase {
	return &ChatUseCase{ai: ai, sessionRepo: sessionRepo, systemLimiter: systemLimiter, userLimiter: userLimiter, metrics: metrics, logger: logger}
}

func (uc *ChatUseCase) Execute(ctx context.Context, in ChatInput) (*domain.ChatResp, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	req := in.toDomain()
	start := time.Now()
	pipe, resp := uc.runPipeline(ctx, req, start, func(string) {})
	if resp == nil {
		genReq := uc.buildGenerateReq(pipe, req)
		gen := uc.ai.Generate(ctx, genReq)
		uc.metrics.ObserveStage("generate", time.Since(start))
		if gen != nil && gen.MetaSource != "inline" {
			uc.ai.EnsureMeta(ctx, genReq, gen)
		}
		uc.updateConversationState(pipe.session, gen.ToolExecs)
		resp = uc.finalize(ctx, pipe, req, gen, start)
		resp.ToolExecs = gen.ToolExecs
	}
	return resp, nil
}

func (uc *ChatUseCase) ExecuteStream(ctx context.Context, in ChatInput) <-chan domain.StreamChunk {
	out := make(chan domain.StreamChunk, 64)
	if err := in.validate(); err != nil {
		out <- domain.StreamChunk{
			Type:  domain.ChunkDone,
			Final: &domain.ChatResp{Reply: err.Error(), Intent: domain.IntentUnknown},
		}
		close(out)
		return out
	}
	req := in.toDomain()
	go func() {
		defer close(out)
		uc.runStream(ctx, req, out)
	}()
	return out
}

func (uc *ChatUseCase) runStream(ctx context.Context, req *domain.ChatReq, out chan<- domain.StreamChunk) {
	start := time.Now()
	send := func(c domain.StreamChunk) {
		select {
		case out <- c:
		case <-ctx.Done():
		}
	}

	pipe, resp := uc.runPipeline(ctx, req, start, func(s string) {
		send(domain.StreamChunk{Type: domain.ChunkStageUpdate, Stage: s})
	})
	if resp != nil {
		if pipe.cacheHit {
			send(domain.StreamChunk{Type: domain.ChunkStageUpdate, Stage: "cache_hit"})
		}
		send(domain.StreamChunk{Type: domain.ChunkTextDelta, Text: resp.Reply})
		send(domain.StreamChunk{Type: domain.ChunkDone, Final: resp})
		return
	}

	send(domain.StreamChunk{Type: domain.ChunkStageUpdate, Stage: "generating"})
	genReq := uc.buildGenerateReq(pipe, req)
	gen := uc.ai.GenerateStream(ctx, genReq, send)
	if gen != nil && gen.MetaSource != "inline" {
		uc.ai.EnsureMeta(ctx, genReq, gen)
	}
	uc.updateConversationState(pipe.session, gen.ToolExecs)
	resp = uc.finalize(ctx, pipe, req, gen, start)
	resp.ToolExecs = gen.ToolExecs
	send(domain.StreamChunk{Type: domain.ChunkDone, Final: resp})
}

type pipelineData struct {
	session   *domain.Session
	intent    *domain.IntentResult
	knowledge []domain.KnowledgeRef
	embed     EmbedResult
	cacheHit  bool
}

// 限频 → 会话加载 → 已转人工守卫 → 关键词转人工 → 语义缓存 → 意图识别 → 意图转人工 → RAG
func (uc *ChatUseCase) runPipeline(ctx context.Context, req *domain.ChatReq, start time.Time, emitStage func(string)) (*pipelineData, *domain.ChatResp) {
	pipe := &pipelineData{}

	// 系统总限流（保护整个服务，Redis 故障时降级放行）
	if limited, err := uc.systemLimiter.Limit(ctx, systemLimitKey); err != nil {
		uc.logger.Warn("系统限流检查失败，降级放行", logger.Error(err))
	} else if limited {
		uc.metrics.IncRateLimited()
		return pipe, &domain.ChatResp{
			Reply:  "系统繁忙，请稍后再试。",
			Intent: domain.IntentUnknown,
		}
	}

	// 用户限频（Redis 滑动窗口，key 按 userID 隔离）
	if limited, err := uc.userLimiter.Limit(ctx, fmt.Sprintf(userLimitKeyFmt, req.UserID)); err != nil {
		uc.logger.Warn("用户限频检查失败，降级放行", logger.Error(err))
	} else if limited {
		uc.metrics.IncRateLimited()
		return pipe, &domain.ChatResp{
			Reply:  "您的消息发送过于频繁，请稍后再试。",
			Intent: domain.IntentUnknown,
		}
	}

	// 会话状态快速检查（只查元信息，不加载消息）
	emitStage("session_check")
	session, err := uc.sessionRepo.LoadSession(ctx, req.SessionID)
	if err != nil {
		return pipe, &domain.ChatResp{Reply: "会话加载失败，请重试。", Intent: domain.IntentUnknown}
	}

	// 已转人工守卫：不走任何 AI 逻辑，直接记录消息后返回
	if session.Status == domain.SessionHuman {
		now := time.Now()
		msgs := []domain.Message{
			{SessionID: session.ID, Role: domain.RoleUser, Content: req.Message, CreatedAt: now},
			{SessionID: session.ID, Role: domain.RoleAssistant, Content: humanWaitReply, Intent: domain.IntentTransferToHuman, CreatedAt: now},
		}
		go func() {
			if err := uc.sessionRepo.AppendMessages(context.Background(), session, msgs); err != nil {
				uc.logger.Error("持久化人工客服阶段消息失败", logger.Error(err))
			}
		}()
		return pipe, &domain.ChatResp{Reply: humanWaitReply, Intent: domain.IntentTransferToHuman}
	}

	// 关键词转人工（需要加载完整会话用于生成交接摘要）
	if isTransferKeyword(req.Message) {
		uc.metrics.IncIntent(domain.IntentTransferToHuman.String())
		msgs, _ := uc.sessionRepo.LoadMessages(ctx, req.SessionID)
		session.Messages = msgs
		pipe.session = session
		return pipe, uc.handleTransfer(ctx, pipe, req)
	}

	// L1: Exact Cache（精确匹配，Redis String，最快，无需加载会话）
	emitStage("l1_cache")
	if reply, hit := uc.ai.ExactCacheLookup(ctx, req.Message); hit {
		uc.metrics.IncCacheHit()
		pipe.cacheHit = true
		go uc.persistCacheHit(req.SessionID, req.Message, reply)
		return pipe, &domain.ChatResp{Reply: reply, Intent: domain.IntentFAQ}
	}

	// Embedding（原始问题向量化，L2 缓存和 RAG 共用）
	emitStage("embedding")
	pipe.embed = uc.ai.Embed(ctx, req.Message)
	if pipe.embed.Err != nil {
		uc.logger.Warn("向量化失败", logger.Error(pipe.embed.Err))
	}

	// L2: Semantic Cache（语义相似度匹配，Milvus + Redis，无需加载会话）
	emitStage("l2_cache")
	if pipe.embed.Err == nil && len(pipe.embed.Vectors) > 0 {
		if reply, hit := uc.ai.SemanticCacheLookup(ctx, pipe.embed.Vectors[0]); hit {
			uc.metrics.IncCacheHit()
			pipe.cacheHit = true
			go uc.persistCacheHit(req.SessionID, req.Message, reply)
			return pipe, &domain.ChatResp{Reply: reply, Intent: domain.IntentFAQ}
		}
	}
	uc.metrics.IncCacheMiss()

	// L3: RAG Retrieval（知识库检索，带缓存）
	emitStage("retrieval")
	if len(pipe.embed.Vectors) > 0 {
		// 先查 RAG 缓存
		if cachedKnowledge, hit := uc.ai.RAGCacheLookup(ctx, pipe.embed.Vectors[0]); hit {
			pipe.knowledge = cachedKnowledge
		} else {
			// 缓存未命中，执行检索
			t := time.Now()
			pipe.knowledge = uc.ai.Retrieve(ctx, req.Message, pipe.embed.Vectors[0], 3)
			uc.metrics.ObserveStage("retrieval", time.Since(t))
			// 异步写入 RAG 缓存
			if len(pipe.knowledge) > 0 {
				go uc.ai.RAGCacheStore(context.Background(), pipe.embed.Vectors[0], pipe.knowledge)
			}
		}
	}

	// 延迟加载会话消息（只在需要 LLM 生成时加载，用于上下文）
	emitStage("session_load")
	msgs, _ := uc.sessionRepo.LoadMessages(ctx, req.SessionID)
	session.Messages = msgs
	pipe.session = session

	// 意图由 LLM 生成时推断，不再提前识别
	pipe.intent = &domain.IntentResult{Type: domain.IntentUnknown}

	return pipe, nil
}

// 后处理
func (uc *ChatUseCase) finalize(
	ctx context.Context, pipe *pipelineData, req *domain.ChatReq,
	gen *domain.GenerationResult, start time.Time,
) *domain.ChatResp {
	// 加免责声明
	gen.Reply = addDisclaimer(gen.Reply, gen.Confidence)

	// 工具调用产生的回复包含实时业务数据，绝不应被缓存
	// gen.Reply 为空（模型未按格式输出）时也不写缓存，避免缓存坏数据
	if len(gen.ToolExecs) == 0 && gen.Confidence >= confidenceHigh && gen.Reply != "" && pipe.embed.Err == nil && len(pipe.embed.Vectors) > 0 {
		// L1: 精确缓存（高置信度回复）
		go uc.ai.ExactCacheStore(context.Background(), req.Message, gen.Reply)
		// L2: 语义缓存（向量匹配）
		go uc.ai.SemanticCacheStore(context.Background(), pipe.embed.Vectors[0], gen.Reply)
	}

	// 低置信度轮数维护
	if gen.Confidence < confidenceLow {
		pipe.session.LowConfidenceTurns++
	} else {
		pipe.session.LowConfidenceTurns = 0
	}

	now := time.Now()
	latencyMs := time.Since(start).Milliseconds()
	uc.metrics.ObserveStage("total", time.Since(start))
	newMsgs := []domain.Message{
		{SessionID: pipe.session.ID, Role: domain.RoleUser, Content: req.Message, CreatedAt: now},
		{SessionID: pipe.session.ID, Role: domain.RoleAssistant, Content: gen.Reply,
			Intent: pipe.intent.Type, Confidence: gen.Confidence, TokensUsed: gen.TokensUsed,
			LatencyMs: latencyMs, CreatedAt: now},
	}

	resp := &domain.ChatResp{
		Reply:              gen.Reply,
		Intent:             pipe.intent.Type,
		Knowledge:          pipe.knowledge,
		SuggestedQuestions: gen.Suggested,
	}

	// 用户生气了 && 低置信度，转人工
	needEscalate := pipe.session.LowConfidenceTurns >= autoEscalateThreshold ||
		gen.Emotion == "angry" || gen.Emotion == "urgent"
	if needEscalate {
		uc.logger.Info("自动转人工",
			logger.String("session", pipe.session.ID),
			logger.String("emotion", gen.Emotion))
		uc.metrics.IncAutoEscalation()
		uc.escalate(ctx, pipe.session, newMsgs, resp)
	} else {
		go uc.persistTurn(context.Background(), pipe.session.Clone(), newMsgs)
	}

	return resp
}

// 用户主动转人工（关键词/意图识别），要先查出 session，被动转已经有session了
func (uc *ChatUseCase) handleTransfer(ctx context.Context, pipe *pipelineData, req *domain.ChatReq) *domain.ChatResp {
	if pipe.session == nil {
		session, err := uc.sessionRepo.LoadSession(ctx, req.SessionID)
		if err != nil {
			uc.logger.Warn("转人工时会话加载失败", logger.Error(err))
			return &domain.ChatResp{Reply: transferReply, Intent: domain.IntentTransferToHuman}
		}
		msgs, _ := uc.sessionRepo.LoadMessages(ctx, req.SessionID)
		session.Messages = msgs
		pipe.session = session
	}

	now := time.Now()
	newMsgs := []domain.Message{
		{SessionID: pipe.session.ID, Role: domain.RoleUser, Content: req.Message, CreatedAt: now},
		{SessionID: pipe.session.ID, Role: domain.RoleAssistant, Content: transferReply, Intent: domain.IntentTransferToHuman, CreatedAt: now},
	}
	resp := &domain.ChatResp{Reply: transferReply, Intent: domain.IntentTransferToHuman}
	uc.escalate(ctx, pipe.session, newMsgs, resp)
	return resp
}

// 转人工统一入口：用户主动转，系统自动转，都调用这个
func (uc *ChatUseCase) escalate(ctx context.Context, session *domain.Session, newMsgs []domain.Message, resp *domain.ChatResp) {
	resp.HandoffSummary = uc.ai.BuildHandoff(ctx, session.RecentMessages(maxWindowSize))
	cp := session.Clone()
	cp.Status = domain.SessionHuman
	cp.LowConfidenceTurns = 0
	go func() {
		// 消息需要全量历史，元信息只需要终态，所以元信息不走 Kafka，只在终态时一次性 FlushSession
		uc.persistTurn(context.Background(), cp, newMsgs)
		if err := uc.sessionRepo.FlushSession(context.Background(), cp); err != nil {
			uc.logger.Error("刷写会话元信息失败", logger.Error(err))
		}
	}()
}

// --- 持久化 --

// 追加消息到 Redis 热层 + Kafka 异步落库
func (uc *ChatUseCase) persistTurn(ctx context.Context, session *domain.Session, newMsgs []domain.Message) {
	session.Messages = append(session.Messages, newMsgs...)
	session.UpdatedAt = time.Now()
	if err := uc.sessionRepo.AppendMessages(ctx, session, newMsgs); err != nil {
		uc.logger.Error("持久化会话失败", logger.Error(err))
	}
}

// 缓存命中时 session 未加载，需分别加载元信息和消息再走 persistTurn
func (uc *ChatUseCase) persistCacheHit(sessionID, userMsg, reply string) {
	session, err := uc.sessionRepo.LoadSession(context.Background(), sessionID)
	if err != nil {
		uc.logger.Warn("缓存命中持久化时会话加载失败", logger.Error(err))
		return
	}
	msgs, _ := uc.sessionRepo.LoadMessages(context.Background(), sessionID)
	session.Messages = msgs
	now := time.Now()
	uc.persistTurn(context.Background(), session, []domain.Message{
		{SessionID: session.ID, Role: domain.RoleUser, Content: userMsg, CreatedAt: now},
		{SessionID: session.ID, Role: domain.RoleAssistant, Content: reply, Intent: domain.IntentFAQ, Confidence: 1.0, CreatedAt: now},
	})
}

// 辅助方法

func (uc *ChatUseCase) buildGenerateReq(pipe *pipelineData, req *domain.ChatReq) GenerateReq {
	return GenerateReq{
		UserID:    req.UserID,
		Message:   req.Message,
		History:   pipe.session.RecentMessages(maxWindowSize),
		Knowledge: pipe.knowledge,
		State:     &pipe.session.ConvFlow,
	}
}

// updateConversationState 解析本轮工具调用结果，更新 session.ConvFlow
// ConvFlow 随 session 在 persistTurn 里一起落 Redis，无需单独保存
func (uc *ChatUseCase) updateConversationState(session *domain.Session, toolExecs []domain.ToolExec) {
	if session == nil || len(toolExecs) == 0 {
		return
	}
	state := &session.ConvFlow
	for _, exec := range toolExecs {
		if exec.Result == "" {
			continue
		}
		switch exec.Name {
		case "search_products":
			var result struct {
				Products []struct {
					ProductID interface{} `json:"product_id"`
					Name      string      `json:"name"`
				} `json:"products"`
			}
			if err := json.Unmarshal([]byte(exec.Result), &result); err == nil {
				list := make([]domain.ProductSummary, 0, len(result.Products))
				for _, p := range result.Products {
					list = append(list, domain.ProductSummary{
						ProductID: fmt.Sprintf("%v", p.ProductID),
						Name:      p.Name,
					})
				}
				state.ProductList = list
				state.CurrentProductID = ""
				state.CurrentProductName = ""
			}

		case "get_product_detail":
			var result struct {
				ProductID interface{} `json:"product_id"`
				Name      string      `json:"name"`
			}
			if err := json.Unmarshal([]byte(exec.Result), &result); err == nil {
				state.CurrentProductID = fmt.Sprintf("%v", result.ProductID)
				state.CurrentProductName = result.Name
			}

		case "add_to_cart":
			var llmArgs struct {
				ProductRef string `json:"product_ref"`
			}
			if err := json.Unmarshal([]byte(exec.Arguments), &llmArgs); err == nil {
				id, name := resolveProductRef(llmArgs.ProductRef, state)
				if id != "" {
					// 更新 current，使"再来一个"→ product_ref="current" 仍然有效
					state.CurrentProductID = id
					state.CurrentProductName = name
				}
			}

		case "create_order":
			var result struct {
				OrderID string `json:"order_id"`
			}
			if err := json.Unmarshal([]byte(exec.Result), &result); err == nil && result.OrderID != "" {
				state.LastOrderID = result.OrderID
			}
		}
	}
}

func addDisclaimer(reply string, confidence float32) string {
	switch {
	case confidence >= confidenceHigh:
		return reply
	case confidence >= confidenceLow:
		return reply + "\n\n（以上信息仅供参考，如需进一步帮助请联系人工客服）"
	default:
		return reply + "\n\n（以上回答可能不够准确，建议您联系人工客服获取更专业的帮助）"
	}
}

func isTransferKeyword(msg string) bool {
	keywords := []string{"转人工", "人工客服", "真人客服", "人工服务", "找客服", "要客服", "连人工", "接人工", "人工", "客服"}
	for _, kw := range keywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
