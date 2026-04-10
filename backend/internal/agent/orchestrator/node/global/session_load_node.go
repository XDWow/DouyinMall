package global

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
)

// SessionLoadNode 加载/创建持久会话，写入共享 *domain.State（SessionID/UserID/TraceID 来自 st.Input 与 st.TraceID）。
// 生产路径由 graph 包注册的 StatePreHandler 调用 PrepareSession（框架已持 state 锁）；Handler 与节点体分离，便于复用与单测领域逻辑。
type SessionLoadNode struct {
	service agentsession.SessionService
}

func NewSessionLoadNode(service agentsession.SessionService) *SessionLoadNode {
	return &SessionLoadNode{service: service}
}

// PrepareSession 把仓储会话与最近历史写入 st，并保留 AccessGuard 已写入的本轮 Session 字段。
func (n *SessionLoadNode) PrepareSession(ctx context.Context, st *domain.State) error {
	if st == nil {
		return fmt.Errorf("state is required")
	}
	guardRawQuery := st.Session.RawQuery
	guardTenantID := st.Session.TenantID
	guardResume := st.Session.ResumeFromCP
	guardErr := st.Session.ErrorCode
	guardFinal := st.Session.FinalAnswer

	sessionID := strings.TrimSpace(st.Input.SessionID)
	if sessionID == "" {
		sessionID = "sess_" + st.TraceID
	}

	persistedSession, recentHistory, err := n.loadOrCreatePersistedSession(ctx, sessionID, st.Input.UserID)
	if err != nil {
		return err
	}

	persistedSlots, currentRefs, pendingSelections := splitPersistedSessionState(persistedSession.Slots)
	workingSlots := cloneSlots(persistedSlots)
	if workingSlots == nil {
		workingSlots = map[string]any{}
	}
	promoteTrustedRefsIntoSlots(workingSlots, currentRefs)

	outSession := clonePersistedSession(persistedSession)
	outSession.Slots = mergePersistedSessionState(workingSlots, currentRefs, pendingSelections)

	ps := outSession
	st.PersistedSession = &ps
	st.Session = outSession

	st.Session.SessionID = sessionID
	st.Session.Messages = append([]*schema.Message(nil), recentHistory...)
	st.Session.Slots = cloneSlots(workingSlots)
	st.Session.CurrentRefs = currentRefs
	st.Session.PendingSelections = clonePendingSelectionsSL(pendingSelections)

	st.EnsureResponse().SessionID = sessionID

	st.Session.RawQuery = guardRawQuery
	st.Session.TenantID = guardTenantID
	st.Session.ResumeFromCP = guardResume
	st.Session.ErrorCode = guardErr
	st.Session.FinalAnswer = guardFinal
	return nil
}

func clonePendingSelectionsSL(input map[string]domain.PendingSelection) map[string]domain.PendingSelection {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]domain.PendingSelection, len(input))
	for key, selection := range input {
		cloned := domain.PendingSelection{Kind: selection.Kind}
		if len(selection.Options) > 0 {
			cloned.Options = make(map[string]string, len(selection.Options))
			for optionKey, optionValue := range selection.Options {
				cloned.Options[optionKey] = optionValue
			}
		}
		out[key] = cloned
	}
	return out
}

func (n *SessionLoadNode) loadOrCreatePersistedSession(
	ctx context.Context,
	sessionID string,
	userID int64,
) (domain.Session, []*schema.Message, error) {
	if strings.TrimSpace(sessionID) == "" {
		return domain.Session{}, nil, fmt.Errorf("session_id is required")
	}
	if userID <= 0 {
		return domain.Session{}, nil, fmt.Errorf("user_id is required")
	}

	if n == nil || n.service == nil {
		return newActiveSession(sessionID, userID), nil, nil
	}

	snapshot, err := n.service.LoadSnapshot(ctx, sessionID)
	if err != nil {
		return domain.Session{}, nil, err
	}
	if snapshot == nil {
		persistedSession := newActiveSession(sessionID, userID)
		if err := n.service.CreateSession(ctx, persistedSession); err != nil {
			return domain.Session{}, nil, err
		}
		return persistedSession, nil, nil
	}
	if snapshot.PersistedSession.UserID != 0 && snapshot.PersistedSession.UserID != userID {
		return domain.Session{}, nil, fmt.Errorf("session user mismatch")
	}

	return clonePersistedSession(snapshot.PersistedSession), n.service.BuildRecentHistory(snapshot.Messages), nil
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

func clonePersistedSession(session domain.Session) domain.Session {
	cloned := session
	cloned.Slots = cloneSlots(session.Slots)
	return cloned
}

func newActiveSession(sessionID string, userID int64) domain.Session {
	now := time.Now()
	return domain.Session{
		SessionID:  sessionID,
		UserID:     userID,
		Status:     domain.SessionStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
		TotalTurns: 0,
	}
}

var _ agentsession.SessionService = (*agentsession.Service)(nil)
