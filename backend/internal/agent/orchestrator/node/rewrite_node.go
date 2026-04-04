package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type RewriteNode struct{ suite *Suite }

func (s *Suite) Rewrite() *RewriteNode { return &RewriteNode{suite: s} }

func (n *RewriteNode) Evaluate(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

func (n *RewriteNode) Identity(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	state.Rewrite = graphstate.RewriteDecision{Query: strings.TrimSpace(support.FirstNonEmpty(state.Session.RawQuery, state.Request.Message)), Reason: "identity"}
	state.Session.RewrittenQuery = state.Rewrite.Query
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

func (n *RewriteNode) BuildPromptInput(ctx context.Context, state *graphstate.ConversationState) (map[string]any, error) {
	graphstate.BindConversationState(ctx, state)
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	return map[string]any{
		"system_text":  n.suite.deps.Prompts.SystemText,
		"history_text": support.HistoryText(graphstate.RecentMessages(state)),
		"message":      state.Session.RawQuery,
		"intent":       string(state.Session.Intent),
	}, nil
}

func (n *RewriteNode) Apply(ctx context.Context, msg *schema.Message) (*graphstate.ConversationState, error) {
	state := graphstate.ConversationStateFromContext(ctx)
	if state == nil {
		return nil, fmt.Errorf("conversation state is missing")
	}
	query := strings.TrimSpace(support.FirstNonEmpty(state.Session.RawQuery, state.Request.Message))
	reason := "identity"
	if msg != nil {
		if parsedQuery, parsedReason, ok := support.ParseRewriteDecision(msg.Content); ok && strings.TrimSpace(parsedQuery) != "" {
			query = strings.TrimSpace(parsedQuery)
			reason = parsedReason
		}
	}
	state.Rewrite = graphstate.RewriteDecision{Query: query, Reason: reason}
	state.Session.RewrittenQuery = query
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
