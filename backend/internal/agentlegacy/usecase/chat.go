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

	transferReply  = "濮濓絽婀稉鐑樺亶鏉烆剚甯存禍鍝勪紣鐎广垺婀囬敍宀冾嚞缁嬪秴鈧?.."
	humanWaitReply = "閹劌鍑℃潻鐐村复娴滃搫浼愮€广垺婀囬敍灞剧Х閹垰鍑￠柅浣芥彧閿涘矁顕粙宥呪偓娆忔礀婢跺秲鈧?

	systemLimitKey  = "agent:system:limit"
	userLimitKeyFmt = "agent:rate:%d" // 閻劍鍩涚紒鏉戝闂勬劖绁?key閿?d 娑?userID
)

// ChatInput Handler 鐏炲倷绱堕崗銉ф畱閸樼喎顫愮拠閿嬬湴閸欏倹鏆熼敍宀€鏁?ChatUseCase 鐠愮喕鐭楅弽锟犵崣閸氬氦娴嗘稉?domain.ChatReq閵?
type ChatInput struct {
	SessionID string
	UserID    int64
	Message   string
}

func (in ChatInput) validate() error {
	if in.SessionID == "" {
		return errors.New("session_id 娑撳秷鍏樻稉铏光敄")
	}
	if in.UserID <= 0 {
		return errors.New("user_id 韫囧懘銆忔径褌绨?0")
	}
	if in.Message == "" {
		return errors.New("濞戝牊浼呴崘鍛啇娑撳秷鍏樻稉铏光敄")
	}
	if len([]rune(in.Message)) > maxMessageLen {
		return errors.New("濞戝牊浼呴崘鍛啇鐡掑懎鍤梹鍨闂勬劕鍩?)
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

// 娑撳鐪伴懕宀冪煑
//
//	session閿涘湯essionRepo + domain閿涘绱版导姘崇樈閻㈢喎鎳￠崨銊︽埂閵嗕焦绉烽幁顖涘瘮娑斿懎瀵查妴浣哥杽娴ｆ捁顔囪箛鍡楃摠閸岊煉绱濇稉宥囩叀闁?LLM 鐎涙ê婀?
//	AIService閿涙矮绗?LLM/Embedding/MCP 娴溿倓绨伴敍灞戒紣閸忕柉鐨熼悽銊ユ儕閻滎垽绱濆ù浣哥础閹恒劑鈧緤绱濇稉宥囩叀闁挻婀?HTTP 鐠囬攱鐪伴崪灞肩窗鐠?
//	ChatUseCase閿涙氨绱幒鎺戠湴閿涘矁鐨?session 閸旂姾娴囨导姘崇樈 閳?鐠?AIService 閻㈢喐鍨氶崶鐐差槻 閳?鐠?session 閹镐椒绠欓崠鏍电礉閹跺﹣琚辨潏鍦煒閸氬牐鎹ｉ弶銉礉婢跺嫮鎮婇梽鎰邦暥/鏉烆兛姹夊?缂傛挸鐡ㄧ粵澶夌瑹閸斅ゎ潐閸?
type ChatUseCase struct {
	ai            *AIService
	sessionRepo   domain.SessionRepo
	systemLimiter ratelimit.Limiter // 缁崵绮洪幀濠氭濞翠緤绱橰edis 濠婃垵濮╃粣妤€褰涢敍灞筋樋鐎圭偘绶ラ崗鍙橀煩閿?
	userLimiter   ratelimit.Limiter // 閻劍鍩涚紒鏉戝闂勬劖绁﹂敍鍦dis 濠婃垵濮╃粣妤€褰涢敍瀹琫y = agent:rate:<userID>閿?
	metrics       *PipelineMetrics
	logger        logger.LoggerV1
}

func NewChatUseCase(
	ai *AIService,
	sessionRepo domain.SessionRepo,
	systemLimiter ratelimit.Limiter, // 缁崵绮洪幀濠氭濞翠緤绱濇导妯哄帥娴滃海鏁ら幋鐑芥濞?
	userLimiter ratelimit.Limiter, // 閻劍鍩涚紒鏉戝闂勬劖绁﹂敍瀹琫y 閸︺劏鐨熼悽銊︽濞夈劌鍙?
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

// 闂勬劙顣?閳?娴兼俺鐦介崝鐘烘祰 閳?瀹歌尪娴嗘禍鍝勪紣鐎瑰牆宕?閳?閸忔娊鏁拠宥堟祮娴滃搫浼?閳?鐠囶厺绠熺紓鎾崇摠 閳?閹板繐娴樼拠鍡楀焼 閳?閹板繐娴樻潪顑挎眽瀹?閳?RAG
func (uc *ChatUseCase) runPipeline(ctx context.Context, req *domain.ChatReq, start time.Time, emitStage func(string)) (*pipelineData, *domain.ChatResp) {
	pipe := &pipelineData{}

	// 缁崵绮洪幀濠氭濞翠緤绱欐穱婵囧Б閺佺繝閲滈張宥呭閿涘edis 閺佸懘娈伴弮鍫曟缁狙勬杹鐞涘矉绱?
	if limited, err := uc.systemLimiter.Limit(ctx, systemLimitKey); err != nil {
		uc.logger.Warn("缁崵绮洪梽鎰ウ濡偓閺屻儱銇戠拹銉礉闂勫秶楠囬弨鎹愵攽", logger.Error(err))
	} else if limited {
		uc.metrics.IncRateLimited()
		return pipe, &domain.ChatResp{
			Reply:  "缁崵绮虹换浣哥箹閿涘矁顕粙宥呮倵閸愬秷鐦妴?,
			Intent: domain.IntentUnknown,
		}
	}

	// 閻劍鍩涢梽鎰邦暥閿涘湩edis 濠婃垵濮╃粣妤€褰涢敍瀹琫y 閹?userID 闂呮梻顬囬敍?
	if limited, err := uc.userLimiter.Limit(ctx, fmt.Sprintf(userLimitKeyFmt, req.UserID)); err != nil {
		uc.logger.Warn("閻劍鍩涢梽鎰邦暥濡偓閺屻儱銇戠拹銉礉闂勫秶楠囬弨鎹愵攽", logger.Error(err))
	} else if limited {
		uc.metrics.IncRateLimited()
		return pipe, &domain.ChatResp{
			Reply:  "閹劎娈戝☉鍫熶紖閸欐垿鈧浇绻冩禍搴暥缁讳緤绱濈拠椋庘棦閸氬骸鍟€鐠囨洏鈧?,
			Intent: domain.IntentUnknown,
		}
	}

	// 娴兼俺鐦介悩鑸碘偓浣告彥闁喐顥呴弻銉礄閸欘亝鐓￠崗鍐т繆閹垽绱濇稉宥呭鏉炶姤绉烽幁顖ょ礆
	emitStage("session_check")
	session, err := uc.sessionRepo.LoadSession(ctx, req.SessionID)
	if err != nil {
		return pipe, &domain.ChatResp{Reply: "娴兼俺鐦介崝鐘烘祰婢惰精瑙﹂敍宀冾嚞闁插秷鐦妴?, Intent: domain.IntentUnknown}
	}

	// 瀹歌尪娴嗘禍鍝勪紣鐎瑰牆宕奸敍姘瑝鐠ч鎹㈡担?AI 闁槒绶敍宀€娲块幒銉唶瑜版洘绉烽幁顖氭倵鏉╂柨娲?
	if session.Status == domain.SessionHuman {
		now := time.Now()
		msgs := []domain.Message{
			{SessionID: session.ID, Role: domain.RoleUser, Content: req.Message, CreatedAt: now},
			{SessionID: session.ID, Role: domain.RoleAssistant, Content: humanWaitReply, Intent: domain.IntentTransferToHuman, CreatedAt: now},
		}
		go func() {
			if err := uc.sessionRepo.AppendMessages(context.Background(), session, msgs); err != nil {
				uc.logger.Error("閹镐椒绠欓崠鏍︽眽瀹搞儱顓归張宥夋▉濞堝灚绉烽幁顖氥亼鐠?, logger.Error(err))
			}
		}()
		return pipe, &domain.ChatResp{Reply: humanWaitReply, Intent: domain.IntentTransferToHuman}
	}

	// 閸忔娊鏁拠宥堟祮娴滃搫浼愰敍鍫ユ付鐟曚礁濮炴潪钘夌暚閺佺繝绱扮拠婵堟暏娴滃海鏁撻幋鎰唉閹恒儲鎲崇憰渚婄礆
	if isTransferKeyword(req.Message) {
		uc.metrics.IncIntent(domain.IntentTransferToHuman.String())
		msgs, _ := uc.sessionRepo.LoadMessages(ctx, req.SessionID)
		session.Messages = msgs
		pipe.session = session
		return pipe, uc.handleTransfer(ctx, pipe, req)
	}

	// L1: Exact Cache閿涘牏绨跨涵顔煎爱闁板稄绱漅edis String閿涘本娓惰箛顐礉閺冪娀娓堕崝鐘烘祰娴兼俺鐦介敍?
	emitStage("l1_cache")
	if reply, hit := uc.ai.ExactCacheLookup(ctx, req.Message); hit {
		uc.metrics.IncCacheHit()
		pipe.cacheHit = true
		go uc.persistCacheHit(req.SessionID, req.Message, reply)
		return pipe, &domain.ChatResp{Reply: reply, Intent: domain.IntentFAQ}
	}

	// Embedding閿涘牆甯慨瀣６妫版ê鎮滈柌蹇撳閿涘2 缂傛挸鐡ㄩ崪?RAG 閸忚京鏁ら敍?
	emitStage("embedding")
	pipe.embed = uc.ai.Embed(ctx, req.Message)
	if pipe.embed.Err != nil {
		uc.logger.Warn("閸氭垿鍣洪崠鏍с亼鐠?, logger.Error(pipe.embed.Err))
	}

	// L2: Semantic Cache閿涘牐顕㈡稊澶屾祲娴肩厧瀹抽崠褰掑帳閿涘ilvus + Redis閿涘本妫ら棁鈧崝鐘烘祰娴兼俺鐦介敍?
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

	// L3: RAG Retrieval閿涘牏鐓＄拠鍡楃氨濡偓缁鳖澁绱濈敮锔剧处鐎涙﹫绱?
	emitStage("retrieval")
	if len(pipe.embed.Vectors) > 0 {
		// 閸忓牊鐓?RAG 缂傛挸鐡?
		if cachedKnowledge, hit := uc.ai.RAGCacheLookup(ctx, pipe.embed.Vectors[0]); hit {
			pipe.knowledge = cachedKnowledge
		} else {
			// 缂傛挸鐡ㄩ張顏勬嚒娑擃叏绱濋幍褑顢戝Λ鈧槐?
			t := time.Now()
			pipe.knowledge = uc.ai.Retrieve(ctx, req.Message, pipe.embed.Vectors[0], 3)
			uc.metrics.ObserveStage("retrieval", time.Since(t))
			// 瀵倹顒為崘娆忓弳 RAG 缂傛挸鐡?
			if len(pipe.knowledge) > 0 {
				go uc.ai.RAGCacheStore(context.Background(), pipe.embed.Vectors[0], pipe.knowledge)
			}
		}
	}

	// 瀵ゆ儼绻滈崝鐘烘祰娴兼俺鐦藉☉鍫熶紖閿涘牆褰ч崷銊╂付鐟?LLM 閻㈢喐鍨氶弮璺哄鏉炴枻绱濋悽銊ょ艾娑撳﹣绗呴弬鍥风礆
	emitStage("session_load")
	msgs, _ := uc.sessionRepo.LoadMessages(ctx, req.SessionID)
	session.Messages = msgs
	pipe.session = session

	// 閹板繐娴橀悽?LLM 閻㈢喐鍨氶弮鑸靛腹閺傤叏绱濇稉宥呭晙閹绘劕澧犵拠鍡楀焼
	pipe.intent = &domain.IntentResult{Type: domain.IntentUnknown}

	return pipe, nil
}

// 閸氬骸顦╅悶?
func (uc *ChatUseCase) finalize(
	ctx context.Context, pipe *pipelineData, req *domain.ChatReq,
	gen *domain.GenerationResult, start time.Time,
) *domain.ChatResp {
	// 閸旂姴鍘ょ拹锝咃紣閺?
	gen.Reply = addDisclaimer(gen.Reply, gen.Confidence)

	// 瀹搞儱鍙跨拫鍐暏娴溠呮晸閻ㄥ嫬娲栨径宥呭瘶閸氼偄鐤勯弮鏈电瑹閸斺剝鏆熼幑顕嗙礉缂佹繀绗夋惔鏃囶潶缂傛挸鐡?
	// gen.Reply 娑撹櫣鈹栭敍鍫熌侀崹瀣弓閹稿鐗稿蹇氱翻閸戠尨绱氶弮鏈电瘍娑撳秴鍟撶紓鎾崇摠閿涘矂浼╅崗宥囩处鐎涙ê娼栭弫鐗堝祦
	if len(gen.ToolExecs) == 0 && gen.Confidence >= confidenceHigh && gen.Reply != "" && pipe.embed.Err == nil && len(pipe.embed.Vectors) > 0 {
		// L1: 缁墽鈥樼紓鎾崇摠閿涘牓鐝純顔讳繆鎼达箑娲栨径宥忕礆
		go uc.ai.ExactCacheStore(context.Background(), req.Message, gen.Reply)
		// L2: 鐠囶厺绠熺紓鎾崇摠閿涘牆鎮滈柌蹇撳爱闁板稄绱?
		go uc.ai.SemanticCacheStore(context.Background(), pipe.embed.Vectors[0], gen.Reply)
	}

	// 娴ｅ海鐤嗘穱鈥冲鏉烆喗鏆熺紒瀛樺Б
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

	// 閻劍鍩涢悽鐔哥毜娴?&& 娴ｅ海鐤嗘穱鈥冲閿涘矁娴嗘禍鍝勪紣
	needEscalate := pipe.session.LowConfidenceTurns >= autoEscalateThreshold ||
		gen.Emotion == "angry" || gen.Emotion == "urgent"
	if needEscalate {
		uc.logger.Info("閼奉亜濮╂潪顑挎眽瀹?,
			logger.String("session", pipe.session.ID),
			logger.String("emotion", gen.Emotion))
		uc.metrics.IncAutoEscalation()
		uc.escalate(ctx, pipe.session, newMsgs, resp)
	} else {
		go uc.persistTurn(context.Background(), pipe.session.Clone(), newMsgs)
	}

	return resp
}

// 閻劍鍩涙稉璇插З鏉烆兛姹夊銉礄閸忔娊鏁拠?閹板繐娴樼拠鍡楀焼閿涘绱濈憰浣稿帥閺屻儱鍤?session閿涘矁顫﹂崝銊ㄦ祮瀹歌尙绮￠張濉籩ssion娴?
func (uc *ChatUseCase) handleTransfer(ctx context.Context, pipe *pipelineData, req *domain.ChatReq) *domain.ChatResp {
	if pipe.session == nil {
		session, err := uc.sessionRepo.LoadSession(ctx, req.SessionID)
		if err != nil {
			uc.logger.Warn("鏉烆兛姹夊銉︽娴兼俺鐦介崝鐘烘祰婢惰精瑙?, logger.Error(err))
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

// 鏉烆兛姹夊銉х埠娑撯偓閸忋儱褰涢敍姘辨暏閹磋渹瀵岄崝銊ㄦ祮閿涘瞼閮寸紒鐔诲殰閸斻劏娴嗛敍宀勫厴鐠嬪啰鏁ゆ潻娆庨嚋
func (uc *ChatUseCase) escalate(ctx context.Context, session *domain.Session, newMsgs []domain.Message, resp *domain.ChatResp) {
	resp.HandoffSummary = uc.ai.BuildHandoff(ctx, session.RecentMessages(maxWindowSize))
	cp := session.Clone()
	cp.Status = domain.SessionHuman
	cp.LowConfidenceTurns = 0
	go func() {
		// 濞戝牊浼呴棁鈧憰浣稿弿闁插繐宸婚崣璇х礉閸忓啩淇婇幁顖氬涧闂団偓鐟曚胶绮撻幀渚婄礉閹碘偓娴犮儱鍘撴穱鈩冧紖娑撳秷铔?Kafka閿涘苯褰ч崷銊х矒閹焦妞傛稉鈧▎鈩冣偓?FlushSession
		uc.persistTurn(context.Background(), cp, newMsgs)
		if err := uc.sessionRepo.FlushSession(context.Background(), cp); err != nil {
			uc.logger.Error("閸掑嘲鍟撴导姘崇樈閸忓啩淇婇幁顖氥亼鐠?, logger.Error(err))
		}
	}()
}

// --- 閹镐椒绠欓崠?--

// 鏉╄棄濮炲☉鍫熶紖閸?Redis 閻戭厼鐪?+ Kafka 瀵倹顒為拃钘夌氨
func (uc *ChatUseCase) persistTurn(ctx context.Context, session *domain.Session, newMsgs []domain.Message) {
	session.Messages = append(session.Messages, newMsgs...)
	session.UpdatedAt = time.Now()
	if err := uc.sessionRepo.AppendMessages(ctx, session, newMsgs); err != nil {
		uc.logger.Error("閹镐椒绠欓崠鏍︾窗鐠囨繂銇戠拹?, logger.Error(err))
	}
}

// 缂傛挸鐡ㄩ崨鎴掕厬閺?session 閺堫亜濮炴潪鏂ょ礉闂団偓閸掑棗鍩嗛崝鐘烘祰閸忓啩淇婇幁顖氭嫲濞戝牊浼呴崘宥堣泲 persistTurn
func (uc *ChatUseCase) persistCacheHit(sessionID, userMsg, reply string) {
	session, err := uc.sessionRepo.LoadSession(context.Background(), sessionID)
	if err != nil {
		uc.logger.Warn("缂傛挸鐡ㄩ崨鎴掕厬閹镐椒绠欓崠鏍ㄦ娴兼俺鐦介崝鐘烘祰婢惰精瑙?, logger.Error(err))
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

// 鏉堝懎濮弬瑙勭《

func (uc *ChatUseCase) buildGenerateReq(pipe *pipelineData, req *domain.ChatReq) GenerateReq {
	return GenerateReq{
		UserID:    req.UserID,
		Message:   req.Message,
		History:   pipe.session.RecentMessages(maxWindowSize),
		Knowledge: pipe.knowledge,
		State:     &pipe.session.ConvFlow,
	}
}

// updateConversationState 鐟欙絾鐎介張顒冪枂瀹搞儱鍙跨拫鍐暏缂佹挻鐏夐敍灞炬纯閺?session.ConvFlow
// ConvFlow 闂?session 閸?persistTurn 闁插奔绔寸挧鐤儰 Redis閿涘本妫ら棁鈧崡鏇犲娣囨繂鐡?
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
					// 閺囧瓨鏌?current閿涘奔濞?閸愬秵娼垫稉鈧稉?閳?product_ref="current" 娴犲秶鍔ч張澶嬫櫏
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
		return reply + "\n\n閿涘牅浜掓稉濠佷繆閹垯绮庢笟娑樺棘閼板喛绱濇俊鍌炴付鏉╂稐绔村銉ュ簻閸斺晞顕懕鏃傞兇娴滃搫浼愮€广垺婀囬敍?
	default:
		return reply + "\n\n閿涘牅浜掓稉濠傛礀缁涙柨褰查懗鎴掔瑝婢剁喎鍣涵顕嗙礉瀵ら缚顔呴幃銊ㄤ粓缁姹夊銉ヮ吂閺堝秷骞忛崣鏍ㄦ纯娑撴挷绗熼惃鍕簻閸斺晪绱?
	}
}

func isTransferKeyword(msg string) bool {
	keywords := []string{"鏉烆兛姹夊?, "娴滃搫浼愮€广垺婀?, "閻喍姹夌€广垺婀?, "娴滃搫浼愰張宥呭", "閹垫儳顓归張?, "鐟曚礁顓归張?, "鏉╃偘姹夊?, "閹恒儰姹夊?, "娴滃搫浼?, "鐎广垺婀?}
	for _, kw := range keywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}


