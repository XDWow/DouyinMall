package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type SessionLoadNode struct{}

func NewSessionLoadNode() *SessionLoadNode { return &SessionLoadNode{} }

func (n *SessionLoadNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	sessionID := strings.TrimSpace(state.Request.SessionID)
	if sessionID == "" {
		sessionID = "sess_" + state.TraceID
		state.Request.SessionID = sessionID
	}
	if state.SessionMeta == nil {
		state.SessionMeta = &domain.Session{
			SessionID: sessionID,
			UserID:    state.Request.UserID,
			Status:    domain.SessionStatusActive,
		}
	}
	if state.SessionMeta.UserID != 0 && state.SessionMeta.UserID != state.Request.UserID {
		return nil, fmt.Errorf("session owner mismatch")
	}
	ss := graphstate.EnsureSessionState(state)
	ss.SessionID = state.SessionMeta.SessionID
	if graphstate.SlotString(state, "order_id") == "" {
		if id := support.MetadataValue(state.Request.Metadata, "order_id", "orderID"); id != "" {
			graphstate.SetSlot(state, "order_id", support.DigitsOnlyID(id))
		}
	}
	if graphstate.SlotString(state, "product_id") == "" {
		if id := support.MetadataValue(state.Request.Metadata, "product_id", "productID"); id != "" {
			graphstate.SetSlot(state, "product_id", support.DigitsOnlyID(id))
		}
	}
	state.EnsureResponse().SessionID = state.SessionMeta.SessionID
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
