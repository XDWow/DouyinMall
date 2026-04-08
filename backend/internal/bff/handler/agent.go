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

type AgentHandler struct {
	agentClient  agentservice.Client
	streamClient agentservice.StreamClient
}

func NewAgentHandler(agentClient agentservice.Client, streamClient agentservice.StreamClient) *AgentHandler {
	return &AgentHandler{agentClient: agentClient, streamClient: streamClient}
}

func (h *AgentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/chat", h.SendMessage)
	rg.POST("/chat/stream", h.SendMessageStream)
	rg.POST("/session", h.CreateSession)
	rg.GET("/sessions", h.ListSessions)
	rg.GET("/history", h.GetChatHistory)
	rg.DELETE("/session", h.ClearSession)
}

type chatReq struct {
	SessionID string `json:"session_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
}

func (h *AgentHandler) SendMessage(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "请求参数错误: " + err.Error()})
		return
	}

	resp, err := h.agentClient.SendMessage(c.Request.Context(), &agentv1.ChatRequest{
		SessionId: req.SessionID,
		UserId:    getUserID(c),
		Message:   req.Message,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "发送消息失败"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: toChatResp(resp)})
}

func (h *AgentHandler) SendMessageStream(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "请求参数错误: " + err.Error()})
		return
	}

	stream, err := h.streamClient.SendMessageStream(c.Request.Context(), &agentv1.ChatRequest{
		SessionId: req.SessionID,
		UserId:    getUserID(c),
		Message:   req.Message,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "建立流式连接失败"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.SSEvent("error", `{"msg":"streaming not supported"}`)
		return
	}

	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}
			writeSSE(c.Writer, flusher, "error", map[string]string{"msg": "流式消息接收失败"})
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

type createSessionReq struct {
	Channel string `json:"channel" binding:"required"`
}

func (h *AgentHandler) CreateSession(c *gin.Context) {
	var req createSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "请求参数错误: " + err.Error()})
		return
	}

	resp, err := h.agentClient.CreateSession(c.Request.Context(), &agentv1.CreateSessionRequest{
		UserId:  getUserID(c),
		Channel: req.Channel,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "创建会话失败"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{"session_id": resp.GetSessionId()}})
}

func (h *AgentHandler) ListSessions(c *gin.Context) {
	limit := int32(10)
	offset := int32(0)
	if v, err := parseInt32Query(c, "limit"); err == nil && v > 0 {
		limit = v
	}
	if v, err := parseInt32Query(c, "offset"); err == nil && v >= 0 {
		offset = v
	}

	resp, err := h.agentClient.ListSessions(c.Request.Context(), &agentv1.ListSessionsRequest{
		UserId: getUserID(c),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "查询会话列表失败"})
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

func (h *AgentHandler) GetChatHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "session_id 不能为空"})
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
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "查询对话历史失败"})
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

func (h *AgentHandler) ClearSession(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "session_id 不能为空"})
		return
	}

	resp, err := h.agentClient.ClearSession(c.Request.Context(), &agentv1.ClearSessionRequest{
		SessionId: sessionID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "清空会话失败"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{"success": resp.GetSuccess()}})
}

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

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
}

func toChatResp(resp *agentv1.ChatResponse) gin.H {
	if resp == nil {
		return gin.H{}
	}

	result := gin.H{
		"reply":               resp.GetReply(),
		"intent":              resp.GetIntent().String(),
		"suggested_questions": resp.GetSuggestedQuestions(),
	}

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
