package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	domainrepo "github.com/XDWow/DouyinMall/backend/internal/agent/domain/repo"
)

type cacheSnapshotWriter interface {
	SaveCacheSnapshot(ctx context.Context, session domain.Session, messages []domain.SessionMessage) error
}

type persistentRoundWriter interface {
	SaveRoundPersistent(ctx context.Context, session domain.Session, userMessage domain.SessionMessage, assistantMessage domain.SessionMessage) error
}

// Service 负责会话读写的应用层编排：
// 1. 读取持久化快照；
// 2. 根据窗口大小裁剪最近历史；
// 3. 持久化一轮对话产生的摘要和消息。
type Service struct {
	repo     domainrepo.SessionRepository
	maxTurns int
}

func NewService(repo domainrepo.SessionRepository, maxTurns int) *Service {
	return &Service{repo: repo, maxTurns: maxTurns}
}

// LoadSnapshot 读取会话的持久化快照。
// 返回值会复制引用类型字段，避免调用方无意间改脏仓储层拿到的数据。
func (s *Service) LoadSnapshot(ctx context.Context, sessionID string) (*Snapshot, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	persistedSession, messages, err := s.repo.Load(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if persistedSession == nil {
		return nil, nil
	}

	return &Snapshot{
		PersistedSession: cloneSession(*persistedSession),
		Messages:         cloneSessionMessages(messages),
	}, nil
}

func (s *Service) CreateSession(ctx context.Context, session domain.Session) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.Create(ctx, session)
}

// BuildRecentHistory 把完整持久化历史裁剪成模型需要的最近窗口。
func (s *Service) BuildRecentHistory(messages []domain.SessionMessage) []*schema.Message {
	return MessagesToSchema(WindowMessages(messages, s.maxTurns))
}

func (s *Service) SaveTurn(
	ctx context.Context,
	session domain.Session,
	userMsg domain.SessionMessage,
	assistantMsg domain.SessionMessage,
) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.SaveRound(ctx, session, userMsg, assistantMsg)
}

func (s *Service) SaveTurnCache(ctx context.Context, session domain.Session, messages []domain.SessionMessage) error {
	if s == nil || s.repo == nil {
		return nil
	}
	writer, ok := s.repo.(cacheSnapshotWriter)
	if !ok {
		return nil
	}
	return writer.SaveCacheSnapshot(ctx, session, WindowMessages(messages, s.maxTurns))
}

func (s *Service) SaveTurnPersistent(
	ctx context.Context,
	session domain.Session,
	userMsg domain.SessionMessage,
	assistantMsg domain.SessionMessage,
) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if writer, ok := s.repo.(persistentRoundWriter); ok {
		return writer.SaveRoundPersistent(ctx, session, userMsg, assistantMsg)
	}
	return s.repo.SaveRound(ctx, session, userMsg, assistantMsg)
}

func (s *Service) AllMessages(ctx context.Context, sessionID string) ([]domain.SessionMessage, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	return s.repo.LoadAllMessages(ctx, sessionID)
}

func (s *Service) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	if s == nil || s.repo == nil {
		return nil, 0, nil
	}
	return s.repo.ListByUser(ctx, userID, limit, offset)
}

func (s *Service) Clear(ctx context.Context, sessionID string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	return s.repo.Clear(ctx, sessionID)
}

func NewUserMessage(sessionID, content string) domain.SessionMessage {
	return domain.SessionMessage{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      domain.RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func NewAssistantMessage(sessionID, content string, intent domain.Intent, confidence float64) domain.SessionMessage {
	return domain.SessionMessage{
		ID:         uuid.NewString(),
		SessionID:  sessionID,
		Role:       domain.RoleAssistant,
		Content:    content,
		Intent:     intent,
		Confidence: confidence,
		CreatedAt:  time.Now(),
	}
}

func Truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

// WindowMessages 按“轮次”裁剪历史。
// 一轮默认按用户消息 + 助手消息两条估算。
func WindowMessages(messages []domain.SessionMessage, maxTurns int) []domain.SessionMessage {
	if maxTurns <= 0 || len(messages) == 0 {
		return messages
	}
	limit := maxTurns * 2
	if len(messages) > limit {
		return append([]domain.SessionMessage(nil), messages[len(messages)-limit:]...)
	}
	return append([]domain.SessionMessage(nil), messages...)
}

// MessagesToSchema 把业务消息转换成模型消息。
func MessagesToSchema(messages []domain.SessionMessage) []*schema.Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, ToSchemaMessage(msg))
	}
	return out
}

func ToSchemaMessage(msg domain.SessionMessage) *schema.Message {
	switch msg.Role {
	case domain.RoleAssistant:
		return schema.AssistantMessage(msg.Content, nil)
	case domain.RoleSystem:
		return schema.SystemMessage(msg.Content)
	case domain.RoleTool:
		return schema.ToolMessage(msg.Content, msg.ID)
	default:
		return schema.UserMessage(msg.Content)
	}
}

func ToSchemaRole(role domain.Role) schema.RoleType {
	switch role {
	case domain.RoleAssistant:
		return schema.Assistant
	case domain.RoleSystem:
		return schema.System
	case domain.RoleTool:
		return schema.Tool
	default:
		return schema.User
	}
}

// FromSchemaMessages 把运行时的模型消息转换成可持久化的业务消息。
func FromSchemaMessages(sessionID string, messages []*schema.Message) []domain.SessionMessage {
	if len(messages) == 0 {
		return nil
	}
	now := time.Now()
	out := make([]domain.SessionMessage, 0, len(messages))
	for _, msg := range messages {
		if msg == nil || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		out = append(out, domain.SessionMessage{
			ID:        uuid.NewString(),
			SessionID: sessionID,
			Role:      FromSchemaRole(msg.Role),
			Content:   msg.Content,
			CreatedAt: now,
		})
	}
	return out
}

func FromSchemaRole(role schema.RoleType) domain.Role {
	switch role {
	case schema.Assistant:
		return domain.RoleAssistant
	case schema.System:
		return domain.RoleSystem
	case schema.Tool:
		return domain.RoleTool
	default:
		return domain.RoleUser
	}
}

func cloneSession(session domain.Session) domain.Session {
	cloned := session
	cloned.Slots = cloneSlots(session.Slots)
	return cloned
}

func cloneSessionMessages(messages []domain.SessionMessage) []domain.SessionMessage {
	if len(messages) == 0 {
		return nil
	}
	return append([]domain.SessionMessage(nil), messages...)
}

func cloneSlots(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}
