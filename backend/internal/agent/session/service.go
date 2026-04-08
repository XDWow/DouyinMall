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
	SaveCacheSnapshot(ctx context.Context, session domain.Session, messages []domain.Message) error
}

type persistentRoundWriter interface {
	SaveRoundPersistent(ctx context.Context, session domain.Session, userMessage domain.Message, assistantMessage domain.Message) error
}

// Service is the session application service. It loads conversation context,
// trims recent history, and persists completed turns.
type Service struct {
	repo     domainrepo.SessionRepository
	maxTurns int
}

func NewService(repo domainrepo.SessionRepository, maxTurns int) *Service {
	return &Service{repo: repo, maxTurns: maxTurns}
}

func (s *Service) LoadSession(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error) {
	if s == nil || s.repo == nil {
		return nil, nil, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}

	meta, messages, err := s.repo.Load(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if meta == nil {
		return nil, nil, nil
	}

	meta.RecentMessages = MessagesToSchema(WindowMessages(messages, s.maxTurns))
	return meta, messages, nil
}

func (s *Service) CreateSession(ctx context.Context, session domain.Session) error {
	if s == nil || s.repo == nil {
		return nil
	}
	session.RecentMessages = nil
	return s.repo.Create(ctx, session)
}

func (s *Service) RecentSchemaMessages(messages []domain.Message) []*schema.Message {
	return MessagesToSchema(WindowMessages(messages, s.maxTurns))
}

func (s *Service) SaveTurn(
	ctx context.Context,
	session domain.Session,
	userMsg domain.Message,
	assistantMsg domain.Message,
) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.SaveRound(ctx, session, userMsg, assistantMsg)
}

func (s *Service) SaveTurnCache(ctx context.Context, session domain.Session, messages []domain.Message) error {
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
	userMsg domain.Message,
	assistantMsg domain.Message,
) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if writer, ok := s.repo.(persistentRoundWriter); ok {
		return writer.SaveRoundPersistent(ctx, session, userMsg, assistantMsg)
	}
	return s.repo.SaveRound(ctx, session, userMsg, assistantMsg)
}

func (s *Service) AllMessages(ctx context.Context, sessionID string) ([]domain.Message, error) {
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

func NewUserMessage(sessionID, content string) domain.Message {
	return domain.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      domain.RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func NewAssistantMessage(sessionID, content string, intent domain.Intent, confidence float64) domain.Message {
	return domain.Message{
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

func WindowMessages(messages []domain.Message, maxTurns int) []domain.Message {
	if maxTurns <= 0 || len(messages) == 0 {
		return messages
	}
	limit := maxTurns * 2
	if len(messages) > limit {
		return append([]domain.Message(nil), messages[len(messages)-limit:]...)
	}
	return append([]domain.Message(nil), messages...)
}

func MessagesToSchema(messages []domain.Message) []*schema.Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, ToSchemaMessage(msg))
	}
	return out
}

func ToSchemaMessage(msg domain.Message) *schema.Message {
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

func FromSchemaMessages(sessionID string, messages []*schema.Message) []domain.Message {
	if len(messages) == 0 {
		return nil
	}
	now := time.Now()
	out := make([]domain.Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		out = append(out, domain.Message{
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
