package state

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func ConversationStateFromContext(ctx context.Context) *ConversationState {
	var state *ConversationState
	_ = compose.ProcessState[*ConversationState](ctx, func(_ context.Context, current *ConversationState) error {
		state = current
		return nil
	})
	return state
}

func BindConversationState(ctx context.Context, next *ConversationState) {
	if next == nil {
		return
	}
	_ = compose.ProcessState[*ConversationState](ctx, func(_ context.Context, current *ConversationState) error {
		if current == nil {
			return nil
		}
		*current = *next
		return nil
	})
}

func EnsureSessionState(state *ConversationState) *SessionState {
	if state == nil {
		return nil
	}
	if state.Session.Slots == nil {
		state.Session.Slots = map[string]any{}
	}
	return &state.Session
}

// RecentMessages 返回最近一段对话消息。
// 这段窗口已经由 memory.Manager 截断过，可以直接用于意图识别和知识检索。
func RecentMessages(state *ConversationState) []*schema.Message {
	if state == nil || len(state.Session.Messages) == 0 {
		return nil
	}
	return state.Session.Messages
}

// SetRecentMessages 替换最近一段对话消息。
// 传入空切片或 nil 时会清空消息窗口。
func SetRecentMessages(state *ConversationState, messages []*schema.Message) {
	if state == nil {
		return
	}
	if len(messages) == 0 {
		state.Session.Messages = nil
		return
	}
	state.Session.Messages = append([]*schema.Message(nil), messages...)
}

func SlotString(state *ConversationState, keys ...string) string {
	if state == nil || len(state.Session.Slots) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := state.Session.Slots[key]; ok {
			if str := strings.TrimSpace(ToString(value)); str != "" {
				return str
			}
		}
	}
	return ""
}

func SetSlot(state *ConversationState, key string, value any) {
	if state == nil || strings.TrimSpace(key) == "" || value == nil {
		return
	}
	current := EnsureSessionState(state)
	current.Slots[key] = value
}

func DeleteSlot(state *ConversationState, key string) {
	if state == nil || len(state.Session.Slots) == 0 {
		return
	}
	delete(state.Session.Slots, key)
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
