package node

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

// ToolRegistryCheck reports whether the named MCP tool is registered.
type ToolRegistryCheck func(ctx context.Context, name string) bool

// ToolPlanApplier writes the tool-call plans into conversation state so the
// downstream toolexec subgraph can execute them.
type ToolPlanApplier func(ctx context.Context, state *graphstate.ConversationState, plans []domain.ToolCallPlan) (*graphstate.ConversationState, error)

// ConversationTurnPersister persists one completed user+assistant turn.
type ConversationTurnPersister func(ctx context.Context, state *graphstate.ConversationState, reply string, intent domain.Intent, confidence float64) error

// AnswerGenerator calls the LLM (sync or streaming) and returns the reply text.
type AnswerGenerator func(ctx context.Context, state *graphstate.ConversationState, messages []*schema.Message) (string, error)

// parseSlotInt64 extracts the first non-empty slot value and parses it as int64.
func parseSlotInt64(state *graphstate.ConversationState, keys ...string) (int64, error) {
	raw := graphstate.SlotString(state, keys...)
	if raw == "" {
		return 0, fmt.Errorf("missing required slot (%v)", keys)
	}
	return strconv.ParseInt(raw, 10, 64)
}
