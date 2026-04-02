//go:build legacy_agent

package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/ratelimit"
)

const (
	maxWindowSize         = 10
	autoEscalateThreshold = 3
	confidenceHigh        = 0.8
	confidenceLow         = 0.5
	maxMessageLen         = 2000

	transferReply  = "姝ｅ湪涓烘偍杞帴浜哄伐瀹㈡湇锛岃绋嶅€?.."
	humanWaitReply = "鎮ㄥ凡杩炴帴浜哄伐瀹㈡湇锛屾秷鎭凡閫佽揪锛岃绋嶅€欏洖澶嶃€?

	systemLimitKey  = "agent:system:limit"
	userLimitKeyFmt = "agent:rate:%d" // 鐢ㄦ埛缁村害闄愭祦 key锛?d 涓?userID
)

// ChatInput Handler 灞備紶鍏ョ殑鍘熷璇锋眰鍙傛暟锛岀敱 ChatUseCase 璐熻矗鏍￠獙鍚庤浆涓?domain.ChatReq銆?
type ChatInput struct {
	SessionID string
	UserID    int64
	Message   string
}

func (in ChatInput) validate() error {
	if in.SessionID == "" {
		return errors.New("session_id 涓嶈兘涓虹┖")
	}
	if in.UserID <= 0 {
		return errors.New("user_id 蹇呴』澶т簬 0")
	}
	if in.Message == "" {
		return errors.New("娑堟伅鍐呭涓嶈兘涓虹┖")
	}
	if len([]rune(in.Message)) > maxMessageLen {
		return errors.New("娑堟伅鍐呭瓒呭嚭闀垮害闄愬埗")
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

// 涓夊眰鑱岃矗
//
//	session锛圫essionRepo + domain锛夛細浼氳瘽鐢熷懡鍛ㄦ湡銆佹秷鎭寔涔呭寲銆佸疄浣撹蹇嗗瓨鍌紝涓嶇煡閬?LLM 瀛樺湪
//	AIService锛氫笌 LLM/Embedding/MCP 浜や簰锛屽伐鍏疯皟鐢ㄥ惊鐜紝娴佸紡鎺ㄩ€侊紝涓嶇煡閬撴湁 HTTP 璇锋眰鍜屼細璇?
//	ChatUseCase锛氱紪鎺掑眰锛岃皟 session 鍔犺浇浼氳瘽 鈫?璋?AIService 鐢熸垚鍥炲 鈫?璋?session 鎸佷箙鍖栵紝鎶婁袱杈圭矘鍚堣捣鏉ワ紝澶勭悊闄愰/杞汉宸?缂撳瓨绛変笟鍔¤鍒?
type ChatUseCase struct {
	ai            *AIService
	sessionRepo   domain.SessionRepo
	systemLimiter ratelimit.Limiter // 绯荤粺鎬婚檺娴侊紙Redis 婊戝姩绐楀彛锛屽瀹炰緥鍏变韩锛?
	userLimiter   ratelimit.Limiter // 鐢ㄦ埛缁村害闄愭祦锛圧edis 婊戝姩绐楀彛锛宬ey = agent:rate:<userID>锛?
	metrics       *PipelineMetrics
	logger        logger.LoggerV1
}

func NewChatUseCase(
	ai *AIService,
	sessionRepo domain.SessionRepo,
	systemLimiter ratelimit.Limiter, // 绯荤粺鎬婚檺娴侊紝浼樺厛浜庣敤鎴烽檺娴?
	userLimiter ratelimit.Limiter, // 鐢ㄦ埛缁村害闄愭祦锛宬ey 鍦ㄨ皟鐢ㄦ椂娉ㄥ叆
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

// 闄愰 鈫?浼氳瘽鍔犺浇 鈫?宸茶浆浜哄伐瀹堝崼 鈫?鍏抽敭璇嶈浆浜哄伐 鈫?璇箟缂撳瓨 鈫?鎰忓浘璇嗗埆 鈫?鎰忓浘杞汉宸?鈫?RAG
func (uc *ChatUseCase) runPipeline(ctx context.Context, req *domain.ChatReq, start time.Time, emitStage func(string)) (*pipelineData, *domain.ChatResp) {
	pipe := &pipelineData{}

	// 绯荤粺鎬婚檺娴侊紙淇濇姢鏁翠釜鏈嶅姟锛孯edis 鏁呴殰鏃堕檷绾ф斁琛岋級
	if limited, err := uc.systemLimiter.Limit(ctx, systemLimitKey); err != nil {
		uc.logger.Warn("绯荤粺闄愭祦妫€鏌ュけ璐ワ紝闄嶇骇鏀捐", logger.Error(err))
	} else if limited {
		uc.metrics.IncRateLimited()
		return pipe, &domain.ChatResp{
			Reply:  "绯荤粺绻佸繖锛岃绋嶅悗鍐嶈瘯銆?,
			Intent: domain.IntentUnknown,
		}
	}

	// 鐢ㄦ埛闄愰锛圧edis 婊戝姩绐楀彛锛宬ey 鎸?userID 闅旂锛?
	if limited, err := uc.userLimiter.Limit(ctx, fmt.Sprintf(userLimitKeyFmt, req.UserID)); err != nil {
		uc.logger.Warn("鐢ㄦ埛闄愰妫€鏌ュけ璐ワ紝闄嶇骇鏀捐", logger.Error(err))
	} else if limited {
		uc.metrics.IncRateLimited()
		return pipe, &domain.ChatResp{
			Reply:  "鎮ㄧ殑娑堟伅鍙戦€佽繃浜庨绻侊紝璇风◢鍚庡啀璇曘€?,
			Intent: domain.IntentUnknown,
		}
	}

	// 浼氳瘽鐘舵€佸揩閫熸鏌ワ紙鍙煡鍏冧俊鎭紝涓嶅姞杞芥秷鎭級
	emitStage("session_check")
	session, err := uc.sessionRepo.LoadSession(ctx, req.SessionID)
	if err != nil {
		return pipe, &domain.ChatResp{Reply: "浼氳瘽鍔犺浇澶辫触锛岃閲嶈瘯銆?, Intent: domain.IntentUnknown}
	}

	// 宸茶浆浜哄伐瀹堝崼锛氫笉璧颁换浣?AI 閫昏緫锛岀洿鎺ヨ褰曟秷鎭悗杩斿洖
	if session.Status == domain.SessionHuman {
		now := time.Now()
		msgs := []domain.Message{
			{SessionID: session.ID, Role: domain.RoleUser, Content: req.Message, CreatedAt: now},
			{SessionID: session.ID, Role: domain.RoleAssistant, Content: humanWaitReply, Intent: domain.IntentTransferToHuman, CreatedAt: now},
		}
		go func() {
			if err := uc.sessionRepo.AppendMessages(context.Background(), session, msgs); err != nil {
				uc.logger.Error("鎸佷箙鍖栦汉宸ュ鏈嶉樁娈垫秷鎭け璐?, logger.Error(err))
			}
		}()
		return pipe, &domain.ChatResp{Reply: humanWaitReply, Intent: domain.IntentTransferToHuman}
	}

	// 鍏抽敭璇嶈浆浜哄伐锛堥渶瑕佸姞杞藉畬鏁翠細璇濈敤浜庣敓鎴愪氦鎺ユ憳瑕侊級
	if isTransferKeyword(req.Message) {
		uc.metrics.IncIntent(domain.IntentTransferToHuman.String())
		msgs, _ := uc.sessionRepo.LoadMessages(ctx, req.SessionID)
		session.Messages = msgs
		pipe.session = session
		return pipe, uc.handleTransfer(ctx, pipe, req)
	}

	// L1: Exact Cache锛堢簿纭尮閰嶏紝Redis String锛屾渶蹇紝鏃犻渶鍔犺浇浼氳瘽锛?
	emitStage("l1_cache")
	if reply, hit := uc.ai.ExactCacheLookup(ctx, req.Message); hit {
		uc.metrics.IncCacheHit()
		pipe.cacheHit = true
		go uc.persistCacheHit(req.SessionID, req.Message, reply)
		return pipe, &domain.ChatResp{Reply: reply, Intent: domain.IntentFAQ}
	}

	// Embedding锛堝師濮嬮棶棰樺悜閲忓寲锛孡2 缂撳瓨鍜?RAG 鍏辩敤锛?
	emitStage("embedding")
	pipe.embed = uc.ai.Embed(ctx, req.Message)
	if pipe.embed.Err != nil {
		uc.logger.Warn("鍚戦噺鍖栧け璐?, logger.Error(pipe.embed.Err))
	}

	// L2: Semantic Cache锛堣涔夌浉浼煎害鍖归厤锛孧ilvus + Redis锛屾棤闇€鍔犺浇浼氳瘽锛?
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

	// L3: RAG Retrieval锛堢煡璇嗗簱妫€绱紝甯︾紦瀛橈級
	emitStage("retrieval")
	if len(pipe.embed.Vectors) > 0 {
		// 鍏堟煡 RAG 缂撳瓨
		if cachedKnowledge, hit := uc.ai.RAGCacheLookup(ctx, pipe.embed.Vectors[0]); hit {
			pipe.knowledge = cachedKnowledge
		} else {
			// 缂撳瓨鏈懡涓紝鎵ц妫€绱?
			t := time.Now()
			pipe.knowledge = uc.ai.Retrieve(ctx, req.Message, pipe.embed.Vectors[0], 3)
			uc.metrics.ObserveStage("retrieval", time.Since(t))
			// 寮傛鍐欏叆 RAG 缂撳瓨
			if len(pipe.knowledge) > 0 {
				go uc.ai.RAGCacheStore(context.Background(), pipe.embed.Vectors[0], pipe.knowledge)
			}
		}
	}

	// 寤惰繜鍔犺浇浼氳瘽娑堟伅锛堝彧鍦ㄩ渶瑕?LLM 鐢熸垚鏃跺姞杞斤紝鐢ㄤ簬涓婁笅鏂囷級
	emitStage("session_load")
	msgs, _ := uc.sessionRepo.LoadMessages(ctx, req.SessionID)
	session.Messages = msgs
	pipe.session = session

	// 鎰忓浘鐢?LLM 鐢熸垚鏃舵帹鏂紝涓嶅啀鎻愬墠璇嗗埆
	pipe.intent = &domain.IntentResult{Type: domain.IntentUnknown}

	return pipe, nil
}

// 鍚庡鐞?
func (uc *ChatUseCase) finalize(
	ctx context.Context, pipe *pipelineData, req *domain.ChatReq,
	gen *domain.GenerationResult, start time.Time,
) *domain.ChatResp {
	// 鍔犲厤璐ｅ０鏄?
	gen.Reply = addDisclaimer(gen.Reply, gen.Confidence)

	// 宸ュ叿璋冪敤浜х敓鐨勫洖澶嶅寘鍚疄鏃朵笟鍔℃暟鎹紝缁濅笉搴旇缂撳瓨
	// gen.Reply 涓虹┖锛堟ā鍨嬫湭鎸夋牸寮忚緭鍑猴級鏃朵篃涓嶅啓缂撳瓨锛岄伩鍏嶇紦瀛樺潖鏁版嵁
	if len(gen.ToolExecs) == 0 && gen.Confidence >= confidenceHigh && gen.Reply != "" && pipe.embed.Err == nil && len(pipe.embed.Vectors) > 0 {
		// L1: 绮剧‘缂撳瓨锛堥珮缃俊搴﹀洖澶嶏級
		go uc.ai.ExactCacheStore(context.Background(), req.Message, gen.Reply)
		// L2: 璇箟缂撳瓨锛堝悜閲忓尮閰嶏級
		go uc.ai.SemanticCacheStore(context.Background(), pipe.embed.Vectors[0], gen.Reply)
	}

	// 浣庣疆淇″害杞暟缁存姢
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

	// 鐢ㄦ埛鐢熸皵浜?&& 浣庣疆淇″害锛岃浆浜哄伐
	needEscalate := pipe.session.LowConfidenceTurns >= autoEscalateThreshold ||
		gen.Emotion == "angry" || gen.Emotion == "urgent"
	if needEscalate {
		uc.logger.Info("鑷姩杞汉宸?,
			logger.String("session", pipe.session.ID),
			logger.String("emotion", gen.Emotion))
		uc.metrics.IncAutoEscalation()
		uc.escalate(ctx, pipe.session, newMsgs, resp)
	} else {
		go uc.persistTurn(context.Background(), pipe.session.Clone(), newMsgs)
	}

	return resp
}

// 鐢ㄦ埛涓诲姩杞汉宸ワ紙鍏抽敭璇?鎰忓浘璇嗗埆锛夛紝瑕佸厛鏌ュ嚭 session锛岃鍔ㄨ浆宸茬粡鏈塻ession浜?
func (uc *ChatUseCase) handleTransfer(ctx context.Context, pipe *pipelineData, req *domain.ChatReq) *domain.ChatResp {
	if pipe.session == nil {
		session, err := uc.sessionRepo.LoadSession(ctx, req.SessionID)
		if err != nil {
			uc.logger.Warn("杞汉宸ユ椂浼氳瘽鍔犺浇澶辫触", logger.Error(err))
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

// 杞汉宸ョ粺涓€鍏ュ彛锛氱敤鎴蜂富鍔ㄨ浆锛岀郴缁熻嚜鍔ㄨ浆锛岄兘璋冪敤杩欎釜
func (uc *ChatUseCase) escalate(ctx context.Context, session *domain.Session, newMsgs []domain.Message, resp *domain.ChatResp) {
	resp.HandoffSummary = uc.ai.BuildHandoff(ctx, session.RecentMessages(maxWindowSize))
	cp := session.Clone()
	cp.Status = domain.SessionHuman
	cp.LowConfidenceTurns = 0
	go func() {
		// 娑堟伅闇€瑕佸叏閲忓巻鍙诧紝鍏冧俊鎭彧闇€瑕佺粓鎬侊紝鎵€浠ュ厓淇℃伅涓嶈蛋 Kafka锛屽彧鍦ㄧ粓鎬佹椂涓€娆℃€?FlushSession
		uc.persistTurn(context.Background(), cp, newMsgs)
		if err := uc.sessionRepo.FlushSession(context.Background(), cp); err != nil {
			uc.logger.Error("鍒峰啓浼氳瘽鍏冧俊鎭け璐?, logger.Error(err))
		}
	}()
}

// --- 鎸佷箙鍖?--

// 杩藉姞娑堟伅鍒?Redis 鐑眰 + Kafka 寮傛钀藉簱
func (uc *ChatUseCase) persistTurn(ctx context.Context, session *domain.Session, newMsgs []domain.Message) {
	session.Messages = append(session.Messages, newMsgs...)
	session.UpdatedAt = time.Now()
	if err := uc.sessionRepo.AppendMessages(ctx, session, newMsgs); err != nil {
		uc.logger.Error("鎸佷箙鍖栦細璇濆け璐?, logger.Error(err))
	}
}

// 缂撳瓨鍛戒腑鏃?session 鏈姞杞斤紝闇€鍒嗗埆鍔犺浇鍏冧俊鎭拰娑堟伅鍐嶈蛋 persistTurn
func (uc *ChatUseCase) persistCacheHit(sessionID, userMsg, reply string) {
	session, err := uc.sessionRepo.LoadSession(context.Background(), sessionID)
	if err != nil {
		uc.logger.Warn("缂撳瓨鍛戒腑鎸佷箙鍖栨椂浼氳瘽鍔犺浇澶辫触", logger.Error(err))
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

// 杈呭姪鏂规硶

func (uc *ChatUseCase) buildGenerateReq(pipe *pipelineData, req *domain.ChatReq) GenerateReq {
	return GenerateReq{
		UserID:    req.UserID,
		Message:   req.Message,
		History:   pipe.session.RecentMessages(maxWindowSize),
		Knowledge: pipe.knowledge,
		State:     &pipe.session.ConvFlow,
	}
}

// updateConversationState 瑙ｆ瀽鏈疆宸ュ叿璋冪敤缁撴灉锛屾洿鏂?session.ConvFlow
// ConvFlow 闅?session 鍦?persistTurn 閲屼竴璧疯惤 Redis锛屾棤闇€鍗曠嫭淇濆瓨
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
					// 鏇存柊 current锛屼娇"鍐嶆潵涓€涓?鈫?product_ref="current" 浠嶇劧鏈夋晥
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
		return reply + "\n\n锛堜互涓婁俊鎭粎渚涘弬鑰冿紝濡傞渶杩涗竴姝ュ府鍔╄鑱旂郴浜哄伐瀹㈡湇锛?
	default:
		return reply + "\n\n锛堜互涓婂洖绛斿彲鑳戒笉澶熷噯纭紝寤鸿鎮ㄨ仈绯讳汉宸ュ鏈嶈幏鍙栨洿涓撲笟鐨勫府鍔╋級"
	}
}

func isTransferKeyword(msg string) bool {
	keywords := []string{"杞汉宸?, "浜哄伐瀹㈡湇", "鐪熶汉瀹㈡湇", "浜哄伐鏈嶅姟", "鎵惧鏈?, "瑕佸鏈?, "杩炰汉宸?, "鎺ヤ汉宸?, "浜哄伐", "瀹㈡湇"}
	for _, kw := range keywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
