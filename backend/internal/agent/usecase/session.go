//go:build legacy_agent

package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// 会话管理
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

// 获取对话历史
func (uc *SessionUseCase) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]domain.Message, int, error) {
	msgs, err := uc.sessionRepo.LoadMessages(ctx, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("load messages: %w", err)
	}
	total := len(msgs)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return msgs[offset:end], total, nil
}

// 获取用户会话列表
func (uc *SessionUseCase) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	return uc.sessionRepo.ListByUser(ctx, userID, limit, offset)
}

// 清空会话
func (uc *SessionUseCase) Clear(ctx context.Context, sessionID string) error {
	return uc.sessionRepo.Clear(ctx, sessionID)
}

// ==================== 人工客服阶段接口（Phase 2） ====================
// 以下接口供人工客服工作台调用，当前仅预留桩
//
// 整体架构：
//   用户始终通过 Agent 服务发送消息（同一个入口，同一个 session）
//   session.Status == Human 时，ChatUseCase 拦截 AI Pipeline，仅存储用户消息
//   人工坐席通过工作台前端查看消息流，调用以下接口写入回复和结束会话
//   消息读写全部经过 Agent 服务，保证一份数据源
//
// 消息流：
//   用户 → Agent.SendMessage → persistUserMessage → 推送给坐席工作台
//   坐席 → Agent.SendHumanReply → 存 assistant 消息 → 推送给用户前端
//   坐席 → Agent.CloseSession → Status=Closed → FlushSession

// SendHumanReply 人工坐席回复用户
// 坐席工作台调用，写入一条 Role=assistant 的消息并推送给用户
func (uc *SessionUseCase) SendHumanReply(ctx context.Context, sessionID string, content string) error {
	session, err := uc.sessionRepo.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if session.Status != domain.SessionHuman {
		return fmt.Errorf("session %s is not in human mode", sessionID)
	}
	msg := domain.Message{
		SessionID: sessionID,
		Role:      domain.RoleAssistant,
		Content:   content,
		CreatedAt: time.Now(),
	}
	return uc.sessionRepo.AppendMessages(ctx, session, []domain.Message{msg})
	// TODO Phase 2: 通过 WebSocket/SSE 推送给用户前端
}

// 结束人工会话
// 坐席点击"结束会话"时调用，刷写终态到 MySQL
func (uc *SessionUseCase) CloseSession(ctx context.Context, sessionID string) error {
	session, err := uc.sessionRepo.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	session.Status = domain.SessionClosed
	session.UpdatedAt = time.Now()
	return uc.sessionRepo.FlushSession(ctx, session)
}
