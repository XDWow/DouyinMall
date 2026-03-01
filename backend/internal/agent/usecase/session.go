package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// SessionUseCase 会话管理
type SessionUseCase struct {
	sessionRepo domain.SessionRepo
}

func NewSessionUseCase(sessionRepo domain.SessionRepo) *SessionUseCase {
	return &SessionUseCase{sessionRepo: sessionRepo}
}

// Create 创建新会话
func (uc *SessionUseCase) Create(ctx context.Context, userID int64, channel string) (*domain.Session, error) {
	session := &domain.Session{
		ID:        fmt.Sprintf("sess_%d_%d", userID, time.Now().UnixMilli()),
		UserID:    userID,
		Channel:   channel,
		Status:    domain.SessionActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := uc.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

// GetHistory 获取对话历史
func (uc *SessionUseCase) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]domain.Message, int, error) {
	session, err := uc.sessionRepo.Load(ctx, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("load session: %w", err)
	}
	total := len(session.Messages)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return session.Messages[offset:end], total, nil
}

// ListSessions 获取用户会话列表
func (uc *SessionUseCase) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	return uc.sessionRepo.ListByUser(ctx, userID, limit, offset)
}

// Clear 清空会话
func (uc *SessionUseCase) Clear(ctx context.Context, sessionID string) error {
	return uc.sessionRepo.Clear(ctx, sessionID)
}
