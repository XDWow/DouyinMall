package handler

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/usecase"
	agentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/agent/v1"
)

type AgentHandler struct {
	chatUC    *usecase.ChatUseCase
	sessionUC *usecase.SessionUseCase
}

func NewAgentHandler(chatUC *usecase.ChatUseCase, sessionUC *usecase.SessionUseCase) *AgentHandler {
	return &AgentHandler{chatUC: chatUC, sessionUC: sessionUC}
}

// 四阶段 Pipeline 入口
func (h *AgentHandler) SendMessage(ctx context.Context, req *agentv1.ChatRequest) (*agentv1.ChatResponse, error) {
	resp, err := h.chatUC.Execute(ctx, usecase.ChatInput{
		SessionID: req.GetSessionId(),
		UserID:    req.GetUserId(),
		Message:   req.GetMessage(),
	})
	if err != nil {
		return nil, err
	}
	return h.toChatResponse(resp), nil
}

// 流式对话 RPC（gRPC Server-Side Streaming）
// 推送时序：STAGE_UPDATE("cache") → STAGE_UPDATE("intent") → STAGE_UPDATE("retrieval")
//
//	→ STAGE_UPDATE("generating") → TEXT_DELTA × N → DONE
func (h *AgentHandler) SendMessageStream(req *agentv1.ChatRequest, stream agentv1.AgentService_SendMessageStreamServer) error {
	ctx := stream.Context()
	chunkCh := h.chatUC.ExecuteStream(ctx, usecase.ChatInput{
		SessionID: req.GetSessionId(),
		UserID:    req.GetUserId(),
		Message:   req.GetMessage(),
	})
	for chunk := range chunkCh {
		if err := stream.Send(h.toStreamChunk(chunk)); err != nil {
			return err
		}
	}
	return nil
}

// 获取对话历史
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

// 创建新会话
func (h *AgentHandler) CreateSession(ctx context.Context, req *agentv1.CreateSessionRequest) (*agentv1.CreateSessionResponse, error) {
	session, err := h.sessionUC.Create(ctx, req.GetUserId(), req.GetChannel())
	if err != nil {
		return nil, err
	}
	return &agentv1.CreateSessionResponse{
		SessionId: session.ID,
	}, nil
}

// 获取用户会话列表
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
		})
	}
	return &agentv1.ListSessionsResponse{
		Sessions: briefs,
		Total:    int32(total),
	}, nil
}

// 清空会话
func (h *AgentHandler) ClearSession(ctx context.Context, req *agentv1.ClearSessionRequest) (*agentv1.ClearSessionResponse, error) {
	err := h.sessionUC.Clear(ctx, req.GetSessionId())
	if err != nil {
		return &agentv1.ClearSessionResponse{Success: false}, err
	}
	return &agentv1.ClearSessionResponse{Success: true}, nil
}

func (h *AgentHandler) toChatResponse(resp *domain.ChatResp) *agentv1.ChatResponse {
	pbResp := &agentv1.ChatResponse{
		Reply:              resp.Reply,
		Intent:             toProtoIntent(resp.Intent),
		SuggestedQuestions: resp.SuggestedQuestions,
	}

	// 知识引用
	for _, ref := range resp.Knowledge {
		pbResp.Knowledge = append(pbResp.Knowledge, &agentv1.KnowledgeRef{
			Id:        ref.ID,
			Title:     ref.Title,
			Content:   ref.Content,
			Category:  ref.Category,
			Relevance: ref.Relevance,
		})
	}

	// Handoff Summary
	if resp.HandoffSummary != nil {
		pbResp.Handoff = toProtoHandoffSummary(resp.HandoffSummary)
	}

	return pbResp
}

func toProtoHandoffSummary(h *domain.HandoffSummary) *agentv1.HandoffSummary {
	if h == nil {
		return nil
	}
	return &agentv1.HandoffSummary{
		CoreIssue:        h.CoreIssue,
		AiActions:        h.AIActions,
		EscalationReason: h.EscalationReason,
		UserEmotion:      h.UserEmotion,
		Entities:         h.Entities,
	}
}

func (h *AgentHandler) toProtoMessage(m domain.Message) *agentv1.Message {
	return &agentv1.Message{
		Id:         m.ID,
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

func toProtoRole(role domain.Role) agentv1.MessageRole {
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

// 将 domain.StreamChunk 转为 proto ChatStreamChunk
func (h *AgentHandler) toStreamChunk(chunk domain.StreamChunk) *agentv1.ChatStreamChunk {
	pb := &agentv1.ChatStreamChunk{
		Stage: chunk.Stage,
		Text:  chunk.Text,
	}
	switch chunk.Type {
	case domain.ChunkStageUpdate:
		pb.Type = agentv1.ChunkType_STAGE_UPDATE
	case domain.ChunkTextDelta:
		pb.Type = agentv1.ChunkType_TEXT_DELTA
	case domain.ChunkDone:
		pb.Type = agentv1.ChunkType_DONE
		if chunk.Final != nil {
			pb.Final = h.toChatResponse(chunk.Final)
		}
	}
	return pb
}
