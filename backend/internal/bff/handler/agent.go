package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	agentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1/agentservice"
	"github.com/gin-gonic/gin"
)

// AgentHandler BFF 灞?HTTP 澶勭悊鍣紝浠ｇ悊鍓嶇璇锋眰鍒?Agent gRPC 寰湇鍔?
type AgentHandler struct {
	agentClient  agentservice.Client
	streamClient agentservice.StreamClient
}

func NewAgentHandler(agentClient agentservice.Client, streamClient agentservice.StreamClient) *AgentHandler {
	return &AgentHandler{agentClient: agentClient, streamClient: streamClient}
}

// RegisterRoutes 娉ㄥ唽璺敱鍒?gin 寮曟搸
func (h *AgentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/chat", h.SendMessage)
	rg.POST("/chat/stream", h.SendMessageStream)
	rg.POST("/session", h.CreateSession)
	rg.GET("/sessions", h.ListSessions)
	rg.GET("/history", h.GetChatHistory)
	rg.DELETE("/session", h.ClearSession)
}

// ==================== 鍚屾瀵硅瘽 ====================

type chatReq struct {
	SessionID string `json:"session_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
}

// SendMessage POST /agent/api/chat 鈥?鍚屾瀵硅瘽
func (h *AgentHandler) SendMessage(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "鍙傛暟閿欒: " + err.Error()})
		return
	}

	userID := getUserID(c)

	resp, err := h.agentClient.SendMessage(c.Request.Context(), &agentv1.ChatRequest{
		SessionId: req.SessionID,
		UserId:    userID,
		Message:   req.Message,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "鏈嶅姟璋冪敤澶辫触"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: toChatResp(resp)})
}

// ==================== 娴佸紡瀵硅瘽锛圫SE锛?===================

// SendMessageStream POST /agent/api/chat/stream 鈥?SSE 娴佸紡瀵硅瘽
// 浜嬩欢绫诲瀷锛歴tage锛堥樁娈垫帹閫侊級銆乨elta锛堟枃鏈閲忥級銆乨one锛堢粨鏉燂紝鎼哄甫瀹屾暣鍝嶅簲锛夈€乪rror
func (h *AgentHandler) SendMessageStream(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "鍙傛暟閿欒: " + err.Error()})
		return
	}

	userID := getUserID(c)

	// 寤虹珛 gRPC server-side streaming
	stream, err := h.streamClient.SendMessageStream(c.Request.Context(), &agentv1.ChatRequest{
		SessionId: req.SessionID,
		UserId:    userID,
		Message:   req.Message,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "寤虹珛娴佸紡杩炴帴澶辫触"})
		return
	}

	// 璁剧疆 SSE 澶?
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 鍏抽棴 Nginx 缂撳啿
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.SSEvent("error", `{"msg":"streaming not supported"}`)
		return
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			writeSSE(c.Writer, flusher, "error", map[string]string{"msg": "娴佸紡璇诲彇澶辫触"})
			break
		}

		switch chunk.GetType() {
		case agentv1.ChunkType_STAGE_UPDATE:
			writeSSE(c.Writer, flusher, "stage", map[string]string{"stage": chunk.GetStage()})
		case agentv1.ChunkType_TEXT_DELTA:
			writeSSE(c.Writer, flusher, "delta", map[string]string{"text": chunk.GetText()})
		case agentv1.ChunkType_DONE:
			writeSSE(c.Writer, flusher, "done", toChatResp(chunk.GetFinal()))
			return
		}
	}
}

// ==================== 浼氳瘽绠＄悊 ====================

type createSessionReq struct {
	Channel string `json:"channel" binding:"required"`
}

// CreateSession POST /agent/api/session 鈥?鍒涘缓鏂颁細璇?
func (h *AgentHandler) CreateSession(c *gin.Context) {
	var req createSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "鍙傛暟閿欒: " + err.Error()})
		return
	}

	userID := getUserID(c)

	resp, err := h.agentClient.CreateSession(c.Request.Context(), &agentv1.CreateSessionRequest{
		UserId:  userID,
		Channel: req.Channel,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "鍒涘缓浼氳瘽澶辫触"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{"session_id": resp.GetSessionId()}})
}

// ListSessions GET /agent/api/sessions?limit=10&offset=0 鈥?浼氳瘽鍒楄〃
func (h *AgentHandler) ListSessions(c *gin.Context) {
	userID := getUserID(c)

	limit := int32(10)
	offset := int32(0)
	if v, err := parseInt32Query(c, "limit"); err == nil && v > 0 {
		limit = v
	}
	if v, err := parseInt32Query(c, "offset"); err == nil && v >= 0 {
		offset = v
	}

	resp, err := h.agentClient.ListSessions(c.Request.Context(), &agentv1.ListSessionsRequest{
		UserId: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "鑾峰彇浼氳瘽鍒楄〃澶辫触"})
		return
	}

	sessions := make([]gin.H, 0, len(resp.GetSessions()))
	for _, s := range resp.GetSessions() {
		sessions = append(sessions, gin.H{
			"session_id":   s.GetSessionId(),
			"last_message": s.GetLastMessage(),
			"status":       s.GetStatus().String(),
			"created_at":   s.GetCreatedAt(),
			"updated_at":   s.GetUpdatedAt(),
		})
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"sessions": sessions,
		"total":    resp.GetTotal(),
	}})
}

// GetChatHistory GET /agent/api/history?session_id=xxx&limit=20&offset=0 鈥?瀵硅瘽鍘嗗彶
func (h *AgentHandler) GetChatHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "session_id 涓嶈兘涓虹┖"})
		return
	}

	limit := int32(20)
	offset := int32(0)
	if v, err := parseInt32Query(c, "limit"); err == nil && v > 0 {
		limit = v
	}
	if v, err := parseInt32Query(c, "offset"); err == nil && v >= 0 {
		offset = v
	}

	resp, err := h.agentClient.GetChatHistory(c.Request.Context(), &agentv1.GetChatHistoryRequest{
		SessionId: sessionID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "鑾峰彇瀵硅瘽鍘嗗彶澶辫触"})
		return
	}

	messages := make([]gin.H, 0, len(resp.GetMessages()))
	for _, m := range resp.GetMessages() {
		messages = append(messages, gin.H{
			"id":         m.GetId(),
			"session_id": m.GetSessionId(),
			"role":       m.GetRole().String(),
			"content":    m.GetContent(),
			"intent":     m.GetIntent().String(),
			"confidence": m.GetConfidence(),
			"created_at": m.GetCreatedAt(),
		})
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"messages": messages,
		"total":    resp.GetTotal(),
	}})
}

// ClearSession DELETE /agent/api/session?session_id=xxx 鈥?娓呯┖浼氳瘽
func (h *AgentHandler) ClearSession(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "session_id 涓嶈兘涓虹┖"})
		return
	}

	resp, err := h.agentClient.ClearSession(c.Request.Context(), &agentv1.ClearSessionRequest{
		SessionId: sessionID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "娓呯┖浼氳瘽澶辫触"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{"success": resp.GetSuccess()}})
}

// ==================== 杈呭姪鍑芥暟 ====================

// getUserID 浠?gin context 涓彁鍙?JWT 瑙ｆ瀽鍚庣殑 user_id
func getUserID(c *gin.Context) int64 {
	claims, exists := c.Get("claims")
	if !exists {
		return 0
	}
	if uc, ok := claims.(*ginx.UserClaims); ok {
		return uc.Id
	}
	return 0
}

func parseInt32Query(c *gin.Context, key string) (int32, error) {
	v := c.Query(key)
	if v == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int32
	_, err := fmt.Sscanf(v, "%d", &n)
	return n, err
}

// writeSSE 鍐欏叆涓€鏉?SSE 浜嬩欢
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
}

// toChatResp 灏?gRPC ChatResponse 杞负鍓嶇鍙嬪ソ鐨?JSON 缁撴瀯
func toChatResp(resp *agentv1.ChatResponse) gin.H {
	if resp == nil {
		return gin.H{}
	}

	result := gin.H{
		"reply":               resp.GetReply(),
		"intent":              resp.GetIntent().String(),
		"suggested_questions": resp.GetSuggestedQuestions(),
	}

	// 鐭ヨ瘑寮曠敤
	if refs := resp.GetKnowledge(); len(refs) > 0 {
		knowledge := make([]gin.H, 0, len(refs))
		for _, ref := range refs {
			knowledge = append(knowledge, gin.H{
				"id":        ref.GetId(),
				"title":     ref.GetTitle(),
				"content":   ref.GetContent(),
				"category":  ref.GetCategory(),
				"relevance": ref.GetRelevance(),
			})
		}
		result["knowledge"] = knowledge
	}

	// 杞汉宸ユ憳瑕?
	if h := resp.GetHandoff(); h != nil {
		result["handoff"] = gin.H{
			"core_issue":        h.GetCoreIssue(),
			"ai_actions":        h.GetAiActions(),
			"escalation_reason": h.GetEscalationReason(),
			"user_emotion":      h.GetUserEmotion(),
			"entities":          h.GetEntities(),
		}
	}

	return result
}


