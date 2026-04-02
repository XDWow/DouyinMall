package usecase

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
)

type SessionUseCase struct {
	runner Runner
}

func NewSessionUseCase(runner Runner) *SessionUseCase {
	return &SessionUseCase{runner: runner}
}

func (uc *SessionUseCase) Create(ctx context.Context, userID int64, channel string) (*dto.Session, error) {
	return uc.runner.CreateSession(ctx, userID, channel)
}

func (uc *SessionUseCase) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]dto.Message, int, error) {
	return uc.runner.GetHistory(ctx, sessionID, limit, offset)
}

func (uc *SessionUseCase) List(ctx context.Context, userID int64, limit, offset int) ([]dto.Session, int, error) {
	return uc.runner.ListSessions(ctx, userID, limit, offset)
}

func (uc *SessionUseCase) Clear(ctx context.Context, sessionID string) error {
	return uc.runner.ClearSession(ctx, sessionID)
}
