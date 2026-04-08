package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
)

type SessionLoadInput struct {
	Request       domain.ChatCommand
	TraceID       string
	ExistingSlots map[string]any
}

type SessionLoadNode struct {
	service agentsession.SessionService
}

func NewSessionLoadNode(service agentsession.SessionService) *SessionLoadNode {
	return &SessionLoadNode{service: service}
}

type SessionLoadResult struct {
	SessionID         string
	SessionMeta       *domain.Session
	RecentMessages    []*schema.Message
	Slots             map[string]any
	CurrentRefs       graphstate.CurrentRefs
	PendingSelections map[string]graphstate.PendingSelection
}

func (n *SessionLoadNode) Invoke(ctx context.Context, input SessionLoadInput) (*SessionLoadResult, error) {
	sessionID := strings.TrimSpace(input.Request.SessionID)
	if sessionID == "" {
		sessionID = "sess_" + input.TraceID
	}

	meta, recentMessages, err := n.ensureSessionMeta(ctx, sessionID, input.Request.UserID)
	if err != nil {
		return nil, err
	}

	persistedSlots, currentRefs, pendingSelections := splitPersistedSessionState(meta.Slots)
	currentRefs = refsFromMetadata(input.Request.Metadata, currentRefs)
	slots := restoreSessionSlots(persistedSlots, input.ExistingSlots, input.Request.Metadata, currentRefs)
	meta.Slots = mergePersistedSessionState(slots, currentRefs, pendingSelections)

	return &SessionLoadResult{
		SessionID:         sessionID,
		SessionMeta:       meta,
		RecentMessages:    recentMessages,
		Slots:             slots,
		CurrentRefs:       currentRefs,
		PendingSelections: pendingSelections,
	}, nil
}

func (n *SessionLoadNode) ensureSessionMeta(
	ctx context.Context,
	sessionID string,
	userID int64,
) (*domain.Session, []*schema.Message, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}
	if userID <= 0 {
		return nil, nil, fmt.Errorf("user_id is required")
	}

	if n == nil || n.service == nil {
		now := time.Now()
		return &domain.Session{
			SessionID:  sessionID,
			UserID:     userID,
			Status:     domain.SessionStatusActive,
			CreatedAt:  now,
			UpdatedAt:  now,
			TotalTurns: 0,
		}, nil, nil
	}

	meta, messages, err := n.service.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if meta == nil {
		now := time.Now()
		meta = &domain.Session{
			SessionID:  sessionID,
			UserID:     userID,
			Status:     domain.SessionStatusActive,
			CreatedAt:  now,
			UpdatedAt:  now,
			TotalTurns: 0,
		}
		if err := n.service.CreateSession(ctx, *meta); err != nil {
			return nil, nil, err
		}
	}
	if meta.UserID != 0 && meta.UserID != userID {
		return nil, nil, fmt.Errorf("session user mismatch")
	}

	cloned := *meta
	cloned.Slots = cloneSlots(meta.Slots)
	return &cloned, n.service.RecentSchemaMessages(messages), nil
}

func restoreSessionSlots(
	persisted map[string]any,
	existing map[string]any,
	metadata map[string]string,
	currentRefs graphstate.CurrentRefs,
) map[string]any {
	slots := cloneSlots(persisted)
	if slots == nil {
		slots = map[string]any{}
	}
	support.MergeSlots(slots, existing)
	support.MergeSlots(slots, extractMetadataSlots(metadata))
	applyTrustedRefsToSlots(slots, currentRefs)
	return slots
}

func cloneSlots(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

var _ agentsession.SessionService = (*agentsession.Service)(nil)
