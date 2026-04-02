package state

import (
	"context"
	"fmt"
	"strings"
)

func ConversationFlowFromContext(ctx context.Context) *FlowContext {
	var flow *FlowContext
	_ = ProcessConversationState(ctx, func(state *ConversationState) error {
		flow = state.Flow
		return nil
	})
	return flow
}

func BindConversationFlow(ctx context.Context, flow *FlowContext) {
	if flow == nil {
		return
	}
	_ = ProcessConversationState(ctx, func(state *ConversationState) error {
		state.Flow = flow
		return nil
	})
}

func EnsureSessionState(flow *FlowContext) *SessionState {
	if flow == nil {
		return nil
	}
	if flow.State.Slots == nil {
		flow.State.Slots = map[string]any{}
	}
	return &flow.State
}

func SlotString(flow *FlowContext, keys ...string) string {
	if flow == nil || len(flow.State.Slots) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := flow.State.Slots[key]; ok {
			if str := strings.TrimSpace(ToString(value)); str != "" {
				return str
			}
		}
	}
	return ""
}

func SetSlot(flow *FlowContext, key string, value any) {
	if flow == nil || strings.TrimSpace(key) == "" || value == nil {
		return
	}
	state := EnsureSessionState(flow)
	state.Slots[key] = value
}

func DeleteSlot(flow *FlowContext, key string) {
	if flow == nil || len(flow.State.Slots) == 0 {
		return
	}
	delete(flow.State.Slots, key)
}

func ToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case interface{ String() string }:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}
