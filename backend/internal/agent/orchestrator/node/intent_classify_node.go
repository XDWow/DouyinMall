package node

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type IntentClassifyNodeDeps struct {
	Prompts *orchestratorprompt.Set
}

type IntentClassifyNode struct{ deps IntentClassifyNodeDeps }

func NewIntentClassifyNode(deps IntentClassifyNodeDeps) *IntentClassifyNode {
	return &IntentClassifyNode{deps: deps}
}

// Invoke is the fast heuristic-only path used when no LLM subgraph is built.
// LLM-based classification runs through the intentclassify subgraph:
// BuildPromptInput -> IntentPromptNode -> IntentModelNode -> Apply.
func (n *IntentClassifyNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	history := graphstate.RecentMessages(state)
	intent := support.HeuristicIntent(state.Session.RawQuery)
	intent.NeedRewrite = support.RequiresRewrite(state.Session.RawQuery, history)
	state.Intent = intent
	state.Session.Intent = intent.Intent
	state.Session.IntentConfidence = intent.Confidence
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

func (n *IntentClassifyNode) BuildPromptInput(ctx context.Context, state *graphstate.ConversationState) (map[string]any, error) {
	graphstate.BindConversationState(ctx, state)
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	return map[string]any{
		"system_text":  n.deps.Prompts.SystemText,
		"history_text": support.HistoryText(graphstate.RecentMessages(state)),
		"message":      state.Session.RawQuery,
	}, nil
}

func (n *IntentClassifyNode) Apply(ctx context.Context, msg *schema.Message) (*graphstate.ConversationState, error) {
	state := graphstate.ConversationStateFromContext(ctx)
	if state == nil {
		return nil, fmt.Errorf("conversation state is missing")
	}
	history := graphstate.RecentMessages(state)
	intent := support.HeuristicIntent(state.Session.RawQuery)
	if msg != nil {
		if parsed, ok := support.ParseIntentDecision(msg.Content); ok {
			intent = parsed
		}
	}
	intent.NeedRewrite = intent.NeedRewrite || support.RequiresRewrite(state.Session.RawQuery, history)
	state.Intent = intent
	state.Session.Intent = intent.Intent
	state.Session.IntentConfidence = intent.Confidence
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
