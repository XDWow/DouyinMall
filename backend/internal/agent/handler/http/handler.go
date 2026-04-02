package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	agentusecase "github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
)

type Handler struct {
	usecase agentusecase.Service
}

func NewHandler(usecase agentusecase.Service) *Handler {
	return &Handler{usecase: usecase}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/chat", h.Chat)
	rg.POST("/chat/stream", h.ChatStream)
	rg.POST("/sessions", h.CreateSession)
	rg.GET("/sessions", h.ListSessions)
	rg.GET("/history", h.GetHistory)
	rg.DELETE("/sessions/:session_id", h.ClearSession)
}

func (h *Handler) Chat(c *gin.Context) {
	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = firstPositive(req.UserID, headerInt64(c, "X-User-ID"))

	resp, err := h.usecase.Chat(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ChatStream(c *gin.Context) {
	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = firstPositive(req.UserID, headerInt64(c, "X-User-ID"))

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "streaming is not supported"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	writer := NewSSEWriter(c.Writer, flusher)
	if _, err := h.usecase.ChatStream(c.Request.Context(), req, writer); err != nil {
		_ = writer.Send(c.Request.Context(), dto.StreamEvent{
			Event: "error",
			Data:  gin.H{"message": err.Error()},
		})
	}
}

func (h *Handler) CreateSession(c *gin.Context) {
	var req struct {
		UserID  int64  `json:"user_id"`
		Channel string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = firstPositive(req.UserID, headerInt64(c, "X-User-ID"))

	resp, err := h.usecase.CreateSession(c.Request.Context(), req.UserID, req.Channel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListSessions(c *gin.Context) {
	userID := firstPositive(queryInt64(c, "user_id"), headerInt64(c, "X-User-ID"))
	limit := int(queryInt64(c, "limit"))
	offset := int(queryInt64(c, "offset"))

	sessions, total, err := h.usecase.ListSessions(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.SessionList{Sessions: sessions, Total: total})
}

func (h *Handler) GetHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	limit := int(queryInt64(c, "limit"))
	offset := int(queryInt64(c, "offset"))

	messages, total, err := h.usecase.GetHistory(c.Request.Context(), sessionID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.HistoryList{Messages: messages, Total: total})
}

func (h *Handler) ClearSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		sessionID = c.Query("session_id")
	}
	if err := h.usecase.ClearSession(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func queryInt64(c *gin.Context, key string) int64 {
	raw := c.Query(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func headerInt64(c *gin.Context, key string) int64 {
	raw := c.GetHeader(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
