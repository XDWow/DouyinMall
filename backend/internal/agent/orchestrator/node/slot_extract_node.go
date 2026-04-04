package node

import (
	"context"
	"fmt"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type SlotExtractNode struct{ suite *Suite }

func (s *Suite) SlotExtract() *SlotExtractNode { return &SlotExtractNode{suite: s} }

func (n *SlotExtractNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	ss := graphstate.EnsureSessionState(state)
	support.MergeSlots(ss.Slots, support.ExtractMetadataSlots(state.Request.Metadata))
	support.MergeSlots(ss.Slots, support.NormalizeEntitySlots(state.Intent.Entities))
	support.MergeSlots(ss.Slots, support.ExtractSlotsFromMessage(ss.RawQuery, ss.Intent))
	if ss.ResumeFromCP {
		ss.AwaitingUser = false
	}
	if ss.AwaitingConfirm {
		ss.AwaitingUser = false
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
