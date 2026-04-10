package global

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// SlotMergeNode：主图内由 IntentAndSlotNode 的 PreHandler 调用 Apply，不再单独注册为图节点。
// Session.Slots = 上轮落盘 + 工具确认后的 CurrentRefs；本轮解析结果不进这里（子图用 ApplyIntentFieldsForTools）。
//
// SlotMergeInput 路由前确定性合并（无 LLM）。
type SlotMergeInput struct {
	ExistingSlots     map[string]any
	CurrentRefs       domain.CurrentRefs
	PendingSelections map[string]domain.PendingSelection
	AwaitingUser      bool
	AwaitingConfirm   bool
	ResumeFromCP      bool
}

// SlotMergeNode 合并：上轮 Slots + CurrentRefs（持久化或 HydrateCurrentRefs）。
type SlotMergeNode struct{}

func NewSlotMergeNode() *SlotMergeNode { return &SlotMergeNode{} }

type SlotMergeResult struct {
	Slots             map[string]any
	CurrentRefs       domain.CurrentRefs
	PendingSelections map[string]domain.PendingSelection
	AwaitingUser      bool
	AwaitingConfirm   bool
}

func (n *SlotMergeNode) Invoke(_ context.Context, input SlotMergeInput) (*SlotMergeResult, error) {
	slots := map[string]any{}
	for key, value := range input.ExistingSlots {
		slots[key] = value
	}

	currentRefs := input.CurrentRefs
	applyTrustedRefsToSlots(slots, currentRefs)

	awaitingUser := input.AwaitingUser
	if input.ResumeFromCP || input.AwaitingConfirm {
		awaitingUser = false
	}

	return &SlotMergeResult{
		Slots:             slots,
		CurrentRefs:       currentRefs,
		PendingSelections: clonePendingSelections(input.PendingSelections),
		AwaitingUser:      awaitingUser,
		AwaitingConfirm:   input.AwaitingConfirm,
	}, nil
}

func (n *SlotMergeNode) Apply(ctx context.Context, state *domain.State) (*domain.State, error) {
	if state == nil {
		return nil, nil
	}

	result, err := n.Invoke(ctx, SlotMergeInput{
		ExistingSlots:     state.Session.Slots,
		CurrentRefs:       state.Session.CurrentRefs,
		PendingSelections: state.Session.PendingSelections,
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
	state.Session.Intent = state.Intent.Intent
	state.Session.IntentConfidence = state.Intent.Confidence
	return state, nil
}
