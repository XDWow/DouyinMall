package usecase

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func sessionOutputFromDomain(session *domain.Session) *SessionOutput {
	if session == nil {
		return &SessionOutput{}
	}
	return &SessionOutput{
		SessionID:   session.SessionID,
		UserID:      session.UserID,
		Status:      session.Status,
		LastMessage: session.LastMessage,
		TotalTurns:  session.TotalTurns,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}
}

func messageOutputFromDomain(message domain.Message) MessageOutput {
	return MessageOutput{
		ID:         message.ID,
		SessionID:  message.SessionID,
		Role:       message.Role,
		Content:    message.Content,
		Intent:     message.Intent,
		Confidence: message.Confidence,
		CreatedAt:  message.CreatedAt,
	}
}
