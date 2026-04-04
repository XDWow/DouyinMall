package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type SessionUseCase struct {
	runtime runtime
}

func NewSessionUseCase(runtime runtime) *SessionUseCase {
	return &SessionUseCase{runtime: runtime}
}

func (uc *SessionUseCase) Create(ctx context.Context, input CreateSessionInput) (*SessionOutput, error) {
	command, err := validateCreateSessionInput(input)
	if err != nil {
		return nil, err
	}
	session, err := uc.runtime.CreateSession(ctx, command.UserID)
	if err != nil {
		return nil, err
	}
	return sessionOutputFromDomain(session), nil
}

func (uc *SessionUseCase) GetHistory(ctx context.Context, input HistoryInput) (*HistoryOutput, error) {
	query, err := validateHistoryInput(input)
	if err != nil {
		return nil, err
	}
	messages, total, err := uc.runtime.GetHistory(ctx, query.SessionID, query.Limit, query.Offset)
	if err != nil {
		return nil, err
	}
	items := make([]MessageOutput, 0, len(messages))
	for _, item := range messages {
		items = append(items, messageOutputFromDomain(item))
	}
	return &HistoryOutput{Messages: items, Total: total}, nil
}

func (uc *SessionUseCase) List(ctx context.Context, input ListSessionsInput) (*SessionListOutput, error) {
	query, err := validateListSessionsInput(input)
	if err != nil {
		return nil, err
	}
	sessions, total, err := uc.runtime.ListSessions(ctx, query.UserID, query.Limit, query.Offset)
	if err != nil {
		return nil, err
	}
	items := make([]SessionOutput, 0, len(sessions))
	for _, item := range sessions {
		items = append(items, *sessionOutputFromDomain(&item))
	}
	return &SessionListOutput{Sessions: items, Total: total}, nil
}

func (uc *SessionUseCase) Clear(ctx context.Context, input ClearSessionInput) error {
	command, err := validateClearSessionInput(input)
	if err != nil {
		return err
	}
	return uc.runtime.ClearSession(ctx, command.SessionID)
}

func validateCreateSessionInput(input CreateSessionInput) (*domain.CreateSessionCommand, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	return &domain.CreateSessionCommand{UserID: input.UserID}, nil
}

func validateHistoryInput(input HistoryInput) (*domain.HistoryQuery, error) {
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	return &domain.HistoryQuery{
		SessionID: sessionID,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func validateListSessionsInput(input ListSessionsInput) (*domain.SessionListQuery, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	return &domain.SessionListQuery{
		UserID: input.UserID,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func validateClearSessionInput(input ClearSessionInput) (*domain.ClearSessionCommand, error) {
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	return &domain.ClearSessionCommand{SessionID: sessionID}, nil
}
