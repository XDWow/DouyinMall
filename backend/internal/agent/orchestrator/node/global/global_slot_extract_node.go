package global

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type GlobalSlotExtractInput struct {
	ExistingSlots     map[string]any
	RequestMetadata   map[string]string
	CurrentRefs       graphstate.CurrentRefs
	PendingSelections map[string]graphstate.PendingSelection
	Intent            domain.Intent
	IntentEntities    map[string]string
	RawQuery          string
	AwaitingUser      bool
	AwaitingConfirm   bool
	ResumeFromCP      bool
}

type GlobalSlotExtractNode struct{}

func NewGlobalSlotExtractNode() *GlobalSlotExtractNode { return &GlobalSlotExtractNode{} }

type GlobalSlotExtractResult struct {
	Slots             map[string]any
	CurrentRefs       graphstate.CurrentRefs
	PendingSelections map[string]graphstate.PendingSelection
	AwaitingUser      bool
	AwaitingConfirm   bool
}

func (n *GlobalSlotExtractNode) Invoke(_ context.Context, input GlobalSlotExtractInput) (*GlobalSlotExtractResult, error) {
	slots := map[string]any{}
	for key, value := range input.ExistingSlots {
		slots[key] = value
	}

	currentRefs := refsFromMetadata(input.RequestMetadata, input.CurrentRefs)
	support.MergeSlots(slots, extractMetadataSlots(input.RequestMetadata))
	support.MergeSlots(slots, normalizeEntitySlots(input.IntentEntities))
	support.MergeSlots(slots, extractSafeSlotsFromMessage(input.RawQuery, input.Intent))
	applyTrustedRefsToSlots(slots, currentRefs)

	awaitingUser := input.AwaitingUser
	if input.ResumeFromCP || input.AwaitingConfirm {
		awaitingUser = false
	}

	return &GlobalSlotExtractResult{
		Slots:             slots,
		CurrentRefs:       currentRefs,
		PendingSelections: clonePendingSelections(input.PendingSelections),
		AwaitingUser:      awaitingUser,
		AwaitingConfirm:   input.AwaitingConfirm,
	}, nil
}

func (n *GlobalSlotExtractNode) Apply(ctx context.Context, state *graphstate.State) (*graphstate.State, error) {
	if state == nil {
		return nil, nil
	}

	result, err := n.Invoke(ctx, GlobalSlotExtractInput{
		ExistingSlots:     state.Session.Slots,
		RequestMetadata:   state.Request.Metadata,
		CurrentRefs:       state.Session.CurrentRefs,
		PendingSelections: state.Session.PendingSelections,
		Intent:            state.Session.Intent,
		IntentEntities:    state.Intent.Entities,
		RawQuery:          state.Session.RawQuery,
		AwaitingUser:      state.Session.AwaitingUser,
		AwaitingConfirm:   state.Session.AwaitingConfirm,
		ResumeFromCP:      state.Session.ResumeFromCP,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return state, nil
	}

	state.Session.Slots = result.Slots
	state.Session.CurrentRefs = result.CurrentRefs
	state.Session.PendingSelections = result.PendingSelections
	state.Session.AwaitingUser = result.AwaitingUser
	state.Session.AwaitingConfirm = result.AwaitingConfirm
	return state, nil
}
