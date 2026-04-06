package node

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// SlotExtractInput 描述槽位抽取阶段的输入。
type SlotExtractInput struct {
	ExistingSlots   map[string]any
	RequestMetadata map[string]string
	Intent          domain.Intent
	IntentEntities  map[string]string
	RawQuery        string
	AwaitingUser    bool
	AwaitingConfirm bool
	ResumeFromCP    bool
}

// SlotExtractNode 负责从元数据、实体和用户问题中合并槽位。
type SlotExtractNode struct{}

func NewSlotExtractNode() *SlotExtractNode { return &SlotExtractNode{} }

type SlotExtractResult struct {
	Slots           map[string]any
	AwaitingUser    bool
	AwaitingConfirm bool
}

// Invoke 输出当前回合可用的槽位集合。
func (n *SlotExtractNode) Invoke(_ context.Context, input SlotExtractInput) (*SlotExtractResult, error) {
	slots := map[string]any{}
	for key, value := range input.ExistingSlots {
		slots[key] = value
	}

	support.MergeSlots(slots, support.ExtractMetadataSlots(input.RequestMetadata))
	support.MergeSlots(slots, support.NormalizeEntitySlots(input.IntentEntities))
	support.MergeSlots(slots, support.ExtractSlotsFromMessage(input.RawQuery, input.Intent))

	awaitingUser := input.AwaitingUser
	if input.ResumeFromCP || input.AwaitingConfirm {
		awaitingUser = false
	}

	return &SlotExtractResult{
		Slots:           slots,
		AwaitingUser:    awaitingUser,
		AwaitingConfirm: input.AwaitingConfirm,
	}, nil
}
