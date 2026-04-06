package node

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

// ToolRegistryCheck 用于判断指定工具是否已经注册。
type ToolRegistryCheck func(ctx context.Context, name string) bool

// ConversationTurnPersister 用于持久化一轮完整会话。
type ConversationTurnPersister func(ctx context.Context, state *graphstate.ConversationState, reply string, intent domain.Intent, confidence float64) error

// AnswerGenerator 用于调用大模型生成最终回答。
type AnswerGenerator func(ctx context.Context, state *graphstate.ConversationState, messages []*schema.Message) (string, error)

// ToolPlanResult 描述节点生成的工具调用计划，以及执行前已经确定的会话结果。
type ToolPlanResult struct {
	Plans         []domain.ToolCallPlan
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
}

func slotString(slots map[string]any, keys ...string) string {
	if len(slots) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := slots[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				return text
			}
		}
	}
	return ""
}

// parseSlotInt64 提取第一个非空槽位并解析成 int64。
func parseSlotInt64(slots map[string]any, keys ...string) (int64, error) {
	raw := slotString(slots, keys...)
	if raw == "" {
		return 0, fmt.Errorf("缺少必填槽位 %v", keys)
	}
	return strconv.ParseInt(raw, 10, 64)
}
