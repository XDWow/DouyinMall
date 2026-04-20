package usecase

import (
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func sessionOutputFromListItem(item domain.SessionListItem) *SessionOutput {
	return &SessionOutput{
		SessionID:   item.Context.SessionID,
		UserID:      item.Context.UserID,
		Status:      item.Meta.Status,
		LastMessage: item.Meta.LastMessage,
		TotalTurns:  item.Meta.TotalTurns,
		CreatedAt:   item.Meta.CreatedAt,
		UpdatedAt:   item.Meta.UpdatedAt,
	}
}

func messageOutputFromDomain(message domain.SessionMessage) MessageOutput {
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
