package node

import (
	"context"
	"fmt"
	"strings"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type FallbackNode struct{}

func NewFallbackNode() *FallbackNode { return &FallbackNode{} }

func (n *FallbackNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	if strings.TrimSpace(state.Session.FinalAnswer) == "" {
		state.Session.FinalAnswer = support.FallbackAnswer(state)
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
