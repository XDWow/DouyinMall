package node

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type ConfirmSummaryNode struct{ suite *Suite }

func (s *Suite) ConfirmSummary() *ConfirmSummaryNode { return &ConfirmSummaryNode{suite: s} }

func (n *ConfirmSummaryNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	reply := strings.TrimSpace(state.Session.FinalAnswer)
	if reply == "" {
		reply = "Please confirm whether to continue the after-sale request."
	}
	resp := state.EnsureResponse()
	resp.Reply = reply
	resp.Intent = state.Session.Intent
	resp.Status = domain.ReplyStatusFallback
	resp.Confidence = 0.9
	if n.suite.deps.Hooks.PersistConversationTurn != nil {
		if err := n.suite.deps.Hooks.PersistConversationTurn(ctx, state, reply, resp.Intent, resp.Confidence); err != nil {
			n.suite.deps.Logger.Warn("persist confirm turn failed", logger.Error(err))
		}
	}
	graphstate.BindConversationState(ctx, state)
	return state, compose.Interrupt(ctx, map[string]any{"confirm": true, "message": reply})
}

