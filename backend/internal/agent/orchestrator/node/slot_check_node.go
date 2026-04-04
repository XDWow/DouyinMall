package node

import (
	"context"
	"fmt"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type SlotCheckNode struct{ suite *Suite }

func (s *Suite) SlotCheck() *SlotCheckNode { return &SlotCheckNode{suite: s} }

func (n *SlotCheckNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	ss := graphstate.EnsureSessionState(state)
	ss.MissingSlots = support.RequiredMissingSlots(state)
	ss.AwaitingUser = len(ss.MissingSlots) > 0
	if ss.AwaitingUser {
		ss.FinalAnswer = support.AskMessageForMissingSlot(state, ss.MissingSlots[0])
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
