package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type RetrieveNode struct{}

func NewRetrieveNode() *RetrieveNode { return &RetrieveNode{} }

func (n *RetrieveNode) PrepareQuery(ctx context.Context, state *graphstate.ConversationState) (string, error) {
	graphstate.BindConversationState(ctx, state)
	if state == nil {
		return "", fmt.Errorf("state is required")
	}
	return strings.TrimSpace(support.FirstNonEmpty(state.Session.RewrittenQuery, state.Session.RawQuery, state.Request.Message)), nil
}

func (n *RetrieveNode) ApplyDocuments(ctx context.Context, docs []*schema.Document) (*graphstate.ConversationState, error) {
	state := graphstate.ConversationStateFromContext(ctx)
	if state == nil {
		return nil, fmt.Errorf("conversation state is missing")
	}
	state.Retrieval.Query = strings.TrimSpace(support.FirstNonEmpty(state.Session.RewrittenQuery, state.Session.RawQuery))
	state.Retrieval.Documents = docs
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
