package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	agentusecase "github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
	agentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1"
)

const timeLayout = "2006-01-02 15:04:05"

type Handler struct {
	usecase agentusecase.Service
}

func NewHandler(usecase agentusecase.Service) *Handler {
	return &Handler{usecase: usecase}
}

func (h *Handler) SendMessage(ctx context.Context, req *agentv1.ChatRequest) (*agentv1.ChatResponse, error) {
	resp, err := h.usecase.Chat(ctx, dto.ChatRequest{
		SessionID: req.GetSessionId(),
		UserID:    req.GetUserId(),
		Message:   req.GetMessage(),
		Channel:   "grpc",
	})
	if err != nil {
		return nil, err
	}
	return toProtoChatResponse(resp, req.GetMessage()), nil
}

func (h *Handler) SendMessageStream(req *agentv1.ChatRequest, stream agentv1.AgentService_SendMessageStreamServer) error {
	resp, err := h.usecase.ChatStream(stream.Context(), dto.ChatRequest{
		SessionID: req.GetSessionId(),
		UserID:    req.GetUserId(),
		Message:   req.GetMessage(),
		Channel:   "grpc",
	}, NewStreamWriter(stream))
	if err != nil {
		return err
	}

	return stream.Send(&agentv1.ChatStreamChunk{
		Type:  agentv1.ChunkType_DONE,
		Final: toProtoChatResponse(resp, req.GetMessage()),
	})
}

func (h *Handler) CreateSession(ctx context.Context, req *agentv1.CreateSessionRequest) (*agentv1.CreateSessionResponse, error) {
	session, err := h.usecase.CreateSession(ctx, req.GetUserId(), req.GetChannel())
	if err != nil {
		return nil, err
	}
	return &agentv1.CreateSessionResponse{SessionId: session.SessionID}, nil
}

func (h *Handler) GetChatHistory(ctx context.Context, req *agentv1.GetChatHistoryRequest) (*agentv1.GetChatHistoryResponse, error) {
	messages, total, err := h.usecase.GetHistory(ctx, req.GetSessionId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, err
	}

	items := make([]*agentv1.Message, 0, len(messages))
	for _, message := range messages {
		items = append(items, toProtoMessage(message))
	}
	return &agentv1.GetChatHistoryResponse{
		Messages: items,
		Total:    int32(total),
	}, nil
}

func (h *Handler) ListSessions(ctx context.Context, req *agentv1.ListSessionsRequest) (*agentv1.ListSessionsResponse, error) {
	sessions, total, err := h.usecase.ListSessions(ctx, req.GetUserId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, err
	}

	items := make([]*agentv1.SessionBrief, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, &agentv1.SessionBrief{
			SessionId:   session.SessionID,
			LastMessage: session.LastMessage,
			Status:      toProtoSessionStatus(session.Status),
			CreatedAt:   formatTime(session.CreatedAt),
			UpdatedAt:   formatTime(session.UpdatedAt),
			TotalTurns:  int32(session.TotalTurns),
		})
	}
	return &agentv1.ListSessionsResponse{
		Sessions: items,
		Total:    int32(total),
	}, nil
}

func (h *Handler) ClearSession(ctx context.Context, req *agentv1.ClearSessionRequest) (*agentv1.ClearSessionResponse, error) {
	err := h.usecase.ClearSession(ctx, req.GetSessionId())
	return &agentv1.ClearSessionResponse{Success: err == nil}, err
}

func toProtoChatResponse(resp *dto.ChatResponse, userMessage string) *agentv1.ChatResponse {
	if resp == nil {
		return &agentv1.ChatResponse{}
	}

	result := &agentv1.ChatResponse{
		Reply:              resp.Reply,
		Intent:             toProtoIntent(resp.Intent, userMessage),
		Knowledge:          make([]*agentv1.KnowledgeRef, 0, len(resp.References)),
		ToolExecs:          make([]*agentv1.ToolExec, 0, len(resp.ToolExecutions)),
		SuggestedQuestions: buildSuggestedQuestions(resp, userMessage),
	}

	for _, ref := range resp.References {
		result.Knowledge = append(result.Knowledge, &agentv1.KnowledgeRef{
			Id:        ref.ID,
			Title:     ref.Title,
			Content:   ref.Snippet,
			Category:  ref.Category,
			Relevance: float32(ref.Score),
		})
	}
	for _, exec := range resp.ToolExecutions {
		result.ToolExecs = append(result.ToolExecs, &agentv1.ToolExec{
			ToolName:  exec.Name,
			Params:    stringifyMap(exec.Arguments),
			Reasoning: exec.Reason,
			Success:   exec.Success,
			Result:    exec.Result,
			Error:     exec.Error,
			LatencyMs: exec.LatencyMs,
		})
	}

	if resp.NeedHandoff {
		result.Handoff = toProtoHandoff(resp, userMessage)
	}
	return result
}

func toProtoHandoff(resp *dto.ChatResponse, userMessage string) *agentv1.HandoffSummary {
	actions := make([]string, 0, len(resp.ToolExecutions)+2)
	if len(resp.References) > 0 {
		actions = append(actions, fmt.Sprintf("retrieved %d knowledge references", len(resp.References)))
	}
	for _, exec := range resp.ToolExecutions {
		actions = append(actions, fmt.Sprintf("executed tool %s", exec.Name))
	}
	if len(actions) == 0 {
		actions = append(actions, "completed controlled workflow analysis")
	}

	entities := map[string]string{}
	if resp.Trace.RewrittenQuery != "" {
		entities["rewritten_query"] = resp.Trace.RewrittenQuery
	}
	if resp.SessionID != "" {
		entities["session_id"] = resp.SessionID
	}

	return &agentv1.HandoffSummary{
		CoreIssue:        summarize(userMessage, 120),
		AiActions:        actions,
		EscalationReason: firstNonEmpty(resp.HandoffReason, string(resp.Status), "manual_review"),
		UserEmotion:      detectEmotion(userMessage),
		Entities:         entities,
	}
}

func toProtoMessage(message dto.Message) *agentv1.Message {
	return &agentv1.Message{
		Id:         message.ID,
		SessionId:  message.SessionID,
		Role:       toProtoRole(message.Role),
		Content:    message.Content,
		Intent:     toProtoIntent(message.Intent, message.Content),
		Confidence: float32(message.Confidence),
		CreatedAt:  formatTime(message.CreatedAt),
	}
}

func toProtoIntent(intent dto.Intent, message string) agentv1.IntentType {
	msg := strings.ToLower(strings.TrimSpace(message))
	switch intent {
	case dto.IntentReturnPolicy, dto.IntentReturnExchangeApply:
		return agentv1.IntentType_INTENT_RETURN
	case dto.IntentInventoryQuery, dto.IntentProductInfo:
		return agentv1.IntentType_INTENT_PRODUCT_INQUIRY
	case dto.IntentFAQ:
		return agentv1.IntentType_INTENT_FAQ
	case dto.IntentProductSearch, dto.IntentAddToCart:
		if containsAny(msg, "promotion", "discount", "coupon", "deal") {
			return agentv1.IntentType_INTENT_PROMOTION
		}
		return agentv1.IntentType_INTENT_PRODUCT_INQUIRY
	case dto.IntentOrderQuery:
		if containsAny(msg, "logistics", "shipping", "delivery", "shipped") {
			return agentv1.IntentType_INTENT_LOGISTICS
		}
		return agentv1.IntentType_INTENT_ORDER_INQUIRY
	case dto.IntentComplaint:
		return agentv1.IntentType_INTENT_COMPLAINT
	case dto.IntentChitchat:
		return agentv1.IntentType_INTENT_CHITCHAT
	case dto.IntentHandoff:
		return agentv1.IntentType_INTENT_TRANSFER_TO_HUMAN
	case dto.IntentFallback, dto.IntentUnknown:
		if containsAny(msg, "payment", "paid", "charge", "billing") {
			return agentv1.IntentType_INTENT_PAYMENT
		}
		return agentv1.IntentType_INTENT_UNKNOWN
	default:
		if containsAny(msg, "return", "refund", "exchange", "after-sale") {
			return agentv1.IntentType_INTENT_RETURN
		}
		if containsAny(msg, "promotion", "discount", "coupon", "deal") {
			return agentv1.IntentType_INTENT_PROMOTION
		}
		return agentv1.IntentType_INTENT_FAQ
	}
}

func toProtoRole(role dto.Role) agentv1.MessageRole {
	switch role {
	case dto.RoleUser:
		return agentv1.MessageRole_ROLE_USER
	case dto.RoleAssistant:
		return agentv1.MessageRole_ROLE_ASSISTANT
	case dto.RoleSystem:
		return agentv1.MessageRole_ROLE_SYSTEM
	case dto.RoleTool:
		return agentv1.MessageRole_ROLE_TOOL
	default:
		return agentv1.MessageRole_ROLE_UNKNOWN
	}
}

func toProtoSessionStatus(status dto.SessionStatus) agentv1.SessionStatus {
	switch status {
	case dto.SessionStatusClosed:
		return agentv1.SessionStatus_SESSION_CLOSED
	case dto.SessionStatusHuman:
		return agentv1.SessionStatus_SESSION_HUMAN
	default:
		return agentv1.SessionStatus_SESSION_ACTIVE
	}
}

func stringifyMap(values map[string]any) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = fmt.Sprint(value)
	}
	return out
}

func summarize(text string, size int) string {
	runes := []rune(text)
	if len(runes) <= size {
		return text
	}
	return string(runes[:size]) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(timeLayout)
}

func buildSuggestedQuestions(resp *dto.ChatResponse, userMessage string) []string {
	if resp == nil {
		return nil
	}

	seen := map[string]bool{}
	out := make([]string, 0, 3)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] || len(out) >= 3 {
			return
		}
		seen[value] = true
		out = append(out, value)
	}

	switch resp.Intent {
	case dto.IntentOrderQuery:
		add("Do you want me to check the logistics progress as well?")
		add("Should I look up the latest status of this order?")
	case dto.IntentProductSearch:
		add("Do you want similar product recommendations?")
		add("Should I narrow the results by price or category?")
	case dto.IntentAddToCart:
		add("Do you want me to confirm the available specifications first?")
		add("Should I recommend matching products too?")
	case dto.IntentReturnPolicy:
		add("Do you want me to explain the refund conditions in more detail?")
		add("Should I walk through the exchange process as well?")
	case dto.IntentComplaint:
		add("Do you want me to prepare a handoff summary for human support?")
	default:
		add("Can you provide a bit more detail so I can continue?")
	}

	for _, ref := range resp.References {
		if strings.TrimSpace(ref.Title) == "" {
			continue
		}
		add("Do you want me to explain " + ref.Title + " in more detail?")
	}

	if resp.NeedHandoff {
		add("Should I prepare the summary for a human agent now?")
	}
	if len(out) == 0 && userMessage != "" {
		add("Do you want me to continue on the same question?")
	}
	return out
}

func detectEmotion(message string) string {
	msg := strings.ToLower(strings.TrimSpace(message))
	switch {
	case containsAny(msg, "angry", "complaint", "bad", "terrible", "frustrated"):
		return "angry"
	case containsAny(msg, "urgent", "asap", "right now", "hurry", "immediately"):
		return "urgent"
	case containsAny(msg, "confused", "not clear", "problem", "issue"):
		return "mild_frustration"
	default:
		return "neutral"
	}
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
