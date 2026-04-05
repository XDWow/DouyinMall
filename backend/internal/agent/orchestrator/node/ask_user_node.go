package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type AskUserNodeDeps struct {
	PersistTurn ConversationTurnPersister
	Logger      logger.LoggerV1
}

type AskUserNode struct{ deps AskUserNodeDeps }

func NewAskUserNode(deps AskUserNodeDeps) *AskUserNode {
	return &AskUserNode{deps: deps}
}

func (n *AskUserNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	reply := strings.TrimSpace(state.Session.FinalAnswer)
	if reply == "" {
		reply = "Please provide the missing information so I can continue."
	}
	resp := state.EnsureResponse()
	resp.Reply = reply
	resp.Intent = state.Session.Intent
	resp.Status = domain.ReplyStatusFallback
	resp.Confidence = support.MaxFloat(state.Session.IntentConfidence, 0.8)
	if n.deps.PersistTurn != nil {
		if err := n.deps.PersistTurn(ctx, state, reply, resp.Intent, resp.Confidence); err != nil {
			n.deps.Logger.Warn("persist interrupted turn failed", logger.Error(err))
		}
	}
	graphstate.BindConversationState(ctx, state)
	return state, compose.Interrupt(ctx, map[string]any{"missing_slots": state.Session.MissingSlots, "question": reply})
}
