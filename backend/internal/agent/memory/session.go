// Package memory provides the session-scoped conversation memory layer for the
// agent.  It adapts the domain SessionRepository into two contracts:
//
//   - [Manager] -?the high-level entry point used by the graph orchestrator
//     and the use-case / transport layers.
//   - Schema helpers -?factory functions that produce eino schema.Message and
//     domain.Message values, keeping role-mapping in one place.
//
// Memory management in eino custom-graph applications follows a simple
// pattern: load history before the graph runs (injected into the LLM prompt),
// save the completed turn after the graph finishes.  Manager encapsulates both
// sides of that contract while supporting a configurable sliding-window that
// limits how many past turns are forwarded to the model.
package memory

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

// Manager is the single entry point for session-scoped conversation memory.
// It wraps a [domainrepo.SessionRepository] and exposes the operations needed
// by the orchestrator and the session use-case without leaking infrastructure
// details.
//
// All methods are nil-safe: if the Manager or its underlying repository is nil
// the methods return zero values without errors (except where an explicit
// session_id is required).
type Manager struct {
	repo domainrepo.SessionRepository
	// maxTurns is the sliding context window forwarded to the LLM.
	// Each "turn" is one user message + one assistant message, so the actual
	// message slice is capped at maxTurns*2.  Zero means no limit.
	maxTurns int
}

// New creates a Manager backed by repo.  maxTurns controls the sliding window
// of history that is included in LLM prompts; pass 0 to keep all messages.
func New(repo domainrepo.SessionRepository, maxTurns int) *Manager {
	return &Manager{repo: repo, maxTurns: maxTurns}
}

// LoadSession loads the session metadata and its full message history.
// It returns (nil, nil, nil) when the session does not exist yet, letting
// callers decide whether to auto-create one.
func (m *Manager) LoadSession(ctx context.Context, sessionID string) (*domain.Session, []domain.Message, error) {
	if m == nil || m.repo == nil {
		return nil, nil, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}
	return m.repo.Load(ctx, sessionID)
}

// CreateSession persists a new session record.
func (m *Manager) CreateSession(ctx context.Context, session domain.Session) error {
	if m == nil || m.repo == nil {
		return nil
	}
	return m.repo.Create(ctx, session)
}

// RecentSchemaMessages windows the supplied domain messages and converts them
// to eino-native []*schema.Message.  This is the form stored directly in
// ConversationState.Session.Messages so every graph node can use the slice
// without any further conversion.
//
// Windowing is governed by the Manager's maxTurns (default 5 turns = 10 msgs).
func (m *Manager) RecentSchemaMessages(messages []domain.Message) []*schema.Message {
	return MessagesToSchema(WindowMessages(messages, m.maxTurns))
}

// SaveTurn atomically persists one user+assistant exchange and updates the
// session metadata (turn count, last message, timestamps).
func (m *Manager) SaveTurn(
	ctx context.Context,
	session domain.Session,
	userMsg domain.Message,
	assistantMsg domain.Message,
) error {
	if m == nil || m.repo == nil {
		return nil
	}
	return m.repo.SaveRound(ctx, session, userMsg, assistantMsg)
}

// AllMessages returns the full (unwindowed) message history for a session.
// Used by the HTTP / gRPC history endpoints.
func (m *Manager) AllMessages(ctx context.Context, sessionID string) ([]domain.Message, error) {
	if m == nil || m.repo == nil {
		return nil, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	return m.repo.LoadAllMessages(ctx, sessionID)
}

// ListSessions returns a paginated list of sessions for a user.
func (m *Manager) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	if m == nil || m.repo == nil {
		return nil, 0, nil
	}
	return m.repo.ListByUser(ctx, userID, limit, offset)
}

// Clear deletes all messages and the session record itself.
func (m *Manager) Clear(ctx context.Context, sessionID string) error {
	if m == nil || m.repo == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	return m.repo.Clear(ctx, sessionID)
}

// NewUserMessage constructs a domain.Message for the user side of a turn.
func NewUserMessage(sessionID, content string) domain.Message {
	return domain.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      domain.RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

// NewAssistantMessage constructs a domain.Message for the assistant side of a
// turn, with optional intent and confidence annotations.
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

// Truncate shortens text to at most limit runes, appending "-? if trimmed.
func Truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

// WindowMessages applies the sliding context window: keep only the last
// maxTurns*2 messages (one user + one assistant per turn).
// maxTurns <= 0 means no limit.
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

// MessagesToSchema converts a slice of domain.Message to eino-native
// []*schema.Message using [ToSchemaMessage] for each element.
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

// ToSchemaMessage converts a single domain.Message to an eino schema.Message.
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

// ToSchemaRole maps a domain Role to an eino schema RoleType.
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
