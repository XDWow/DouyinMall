package handler

import (
	"context"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
	agentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1"
)

// AgentHandler Kitex gRPC 处理器，映射 proto RPC → UseCase
type AgentHandler struct {
	chatUC    *usecase.ChatUseCase
	sessionUC *usecase.SessionUseCase
}

func NewAgentHandler(chatUC *usecase.ChatUseCase, sessionUC *usecase.SessionUseCase) *AgentHandler {
	return &AgentHandler{chatUC: chatUC, sessionUC: sessionUC}
}

// SendMessage 四阶段 Pipeline 入口
func (h *AgentHandler) SendMessage(ctx context.Context, req *agentv1.ChatRequest) (*agentv1.ChatResponse, error) {
	resp, err := h.chatUC.Execute(ctx, &domain.ChatReq{
		SessionID: req.GetSessionId(),
		UserID:    req.GetUserId(),
		Message:   req.GetMessage(),
		Channel:   req.GetChannel(),
	})
	if err != nil {
		return nil, err
	}
	return h.toChatResponse(resp), nil
}

// GetChatHistory 获取对话历史
func (h *AgentHandler) GetChatHistory(ctx context.Context, req *agentv1.GetChatHistoryRequest) (*agentv1.GetChatHistoryResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	msgs, total, err := h.sessionUC.GetHistory(ctx, req.GetSessionId(), limit, int(req.GetOffset()))
	if err != nil {
		return nil, err
	}

	pbMsgs := make([]*agentv1.Message, 0, len(msgs))
	for _, m := range msgs {
		pbMsgs = append(pbMsgs, h.toProtoMessage(m))
	}
	return &agentv1.GetChatHistoryResponse{
		Messages: pbMsgs,
		Total:    int32(total),
	}, nil
}

// CreateSession 创建新会话
func (h *AgentHandler) CreateSession(ctx context.Context, req *agentv1.CreateSessionRequest) (*agentv1.CreateSessionResponse, error) {
	session, err := h.sessionUC.Create(ctx, req.GetUserId(), req.GetChannel())
	if err != nil {
		return nil, err
	}
	return &agentv1.CreateSessionResponse{
		SessionId: session.ID,
	}, nil
}

// ListSessions 获取用户会话列表
func (h *AgentHandler) ListSessions(ctx context.Context, req *agentv1.ListSessionsRequest) (*agentv1.ListSessionsResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 10
	}
	sessions, total, err := h.sessionUC.ListSessions(ctx, req.GetUserId(), limit, int(req.GetOffset()))
	if err != nil {
		return nil, err
	}

	briefs := make([]*agentv1.SessionBrief, 0, len(sessions))
	for _, s := range sessions {
		briefs = append(briefs, &agentv1.SessionBrief{
			SessionId:   s.ID,
			LastMessage: s.LastMessagePreview(),
			Status:      toProtoSessionStatus(s.Status),
			CreatedAt:   s.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   s.UpdatedAt.Format("2006-01-02 15:04:05"),
			TotalTurns:  int32(s.TotalTurns),
		})
	}
	return &agentv1.ListSessionsResponse{
		Sessions: briefs,
		Total:    int32(total),
	}, nil
}

// ClearSession 清空会话
func (h *AgentHandler) ClearSession(ctx context.Context, req *agentv1.ClearSessionRequest) (*agentv1.ClearSessionResponse, error) {
	err := h.sessionUC.Clear(ctx, req.GetSessionId())
	if err != nil {
		return &agentv1.ClearSessionResponse{Success: false}, err
	}
	return &agentv1.ClearSessionResponse{Success: true}, nil
}

// ==================== 转换函数 ====================

func (h *AgentHandler) toChatResponse(resp *domain.ChatResp) *agentv1.ChatResponse {
	pbResp := &agentv1.ChatResponse{
		Reply:      resp.Reply,
		Intent:     toProtoIntent(resp.Intent),
		Confidence: resp.Confidence,
	}

	// 知识引用
	for _, ref := range resp.References {
		pbResp.Knowledge = append(pbResp.Knowledge, &agentv1.KnowledgeRef{
			Id:        ref.ID,
			Title:     ref.Title,
			Snippet:   ref.Snippet,
			Category:  ref.Category,
			Relevance: ref.Relevance,
		})
	}

	// Pipeline 调试信息
	if resp.Debug != nil {
		pbResp.Debug = &agentv1.PipelineDebug{
			IntentMs:       resp.Debug.IntentMs,
			EmbedMs:        resp.Debug.EmbedMs,
			VectorSearchMs: resp.Debug.VectorMs,
			RerankMs:       resp.Debug.RerankMs,
			GenerateMs:     resp.Debug.GenerateMs,
			ToolMs:         resp.Debug.ToolMs,
			TotalMs:        resp.Debug.TotalMs,
			KnowledgeHits:  int32(resp.Debug.KnowledgeHits),
			CacheHit:       resp.Debug.CacheHit,
			RewrittenQuery: resp.Debug.RewrittenQuery,
		}
	}

	return pbResp
}

func (h *AgentHandler) toProtoMessage(m domain.Message) *agentv1.Message {
	return &agentv1.Message{
		Id:         strconv.FormatInt(m.ID, 10),
		SessionId:  m.SessionID,
		Role:       toProtoRole(m.Role),
		Content:    m.Content,
		Intent:     toProtoIntent(m.Intent),
		Confidence: m.Confidence,
		CreatedAt:  m.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toProtoIntent(intent domain.IntentType) agentv1.IntentType {
	intentMap := map[domain.IntentType]agentv1.IntentType{
		domain.IntentUnknown:         agentv1.IntentType_INTENT_UNKNOWN,
		domain.IntentFAQ:             agentv1.IntentType_INTENT_FAQ,
		domain.IntentProductInquiry:  agentv1.IntentType_INTENT_PRODUCT_INQUIRY,
		domain.IntentOrderInquiry:    agentv1.IntentType_INTENT_ORDER_INQUIRY,
		domain.IntentLogistics:       agentv1.IntentType_INTENT_LOGISTICS,
		domain.IntentPayment:         agentv1.IntentType_INTENT_PAYMENT,
		domain.IntentReturn:          agentv1.IntentType_INTENT_RETURN,
		domain.IntentComplaint:       agentv1.IntentType_INTENT_COMPLAINT,
		domain.IntentPromotion:       agentv1.IntentType_INTENT_PROMOTION,
		domain.IntentChitchat:        agentv1.IntentType_INTENT_CHITCHAT,
		domain.IntentTransferToHuman: agentv1.IntentType_INTENT_TRANSFER_TO_HUMAN,
	}
	if v, ok := intentMap[intent]; ok {
		return v
	}
	return agentv1.IntentType_INTENT_UNKNOWN
}

func toProtoRole(role string) agentv1.MessageRole {
	switch role {
	case domain.RoleUser:
		return agentv1.MessageRole_ROLE_USER
	case domain.RoleAssistant:
		return agentv1.MessageRole_ROLE_ASSISTANT
	case domain.RoleSystem:
		return agentv1.MessageRole_ROLE_SYSTEM
	case domain.RoleTool:
		return agentv1.MessageRole_ROLE_TOOL
	default:
		return agentv1.MessageRole_ROLE_UNKNOWN
	}
}

func toProtoSessionStatus(status domain.SessionStatus) agentv1.SessionStatus {
	switch status {
	case domain.SessionActive:
		return agentv1.SessionStatus_SESSION_ACTIVE
	case domain.SessionClosed:
		return agentv1.SessionStatus_SESSION_CLOSED
	case domain.SessionHuman:
		return agentv1.SessionStatus_SESSION_HUMAN
	default:
		return agentv1.SessionStatus_SESSION_ACTIVE
	}
}
