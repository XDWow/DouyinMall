package aftersalesapply

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type assistInput struct {
	UserMessage    string
	RewrittenQuery string
	History        []*schema.Message
	CurrentOrder   string
	OrderList      []string
	SlotsText      string
}

func assistInputFromState(st *domain.State) (assistInput, error) {
	if st == nil || st.Input == nil || st.Session == nil {
		return assistInput{}, fmt.Errorf("state input/session is required")
	}
	return assistInput{
		UserMessage:    strings.TrimSpace(st.Input.Message),
		RewrittenQuery: strings.TrimSpace(st.RewrittenQuery),
		History:        subgraphcommon.HistoryMessages(st.Session.RecentMessages),
		CurrentOrder:   strings.TrimSpace(st.Session.CurrentOrder),
		OrderList:      append([]string(nil), st.Session.OrderList...),
		SlotsText:      subgraphcommon.RenderSlotsContext(st.Session.Slots),
	}, nil
}

func runAgentAssist(ctx context.Context, agent *sharednode.SubgraphAgent, in assistInput) (*domain.ChatResult, error) {
	if agent == nil || !agent.Enabled() {
		return nil, nil
	}

	finalText, _, err := agent.Run(ctx, sharednode.SubgraphAgentInput{
		ToolNames:    []string{"get_order", "list_user_orders", "query_order"},
		SkillNames:   []string{"aftersale_apply"},
		SlotsContext: buildAssistSlotsContext(in.CurrentOrder, in.OrderList, in.SlotsText),
		UserQuery:    support.FirstNonEmpty(in.RewrittenQuery, in.UserMessage),
		History:      in.History,
		SystemHint:   agentPrompt,
	})
	if err != nil {
		return nil, err
	}

	decision := subgraphcommon.ParseWriteAssistDecision(finalText)
	if err := subgraphcommon.ApplySlotsPatch(ctx, decision.SlotsPatch); err != nil {
		return nil, err
	}

	switch decision.Mode {
	case "ready":
		return nil, nil
	case "answer", "handoff":
		return &domain.ChatResult{
			Intent:        domain.IntentAftersalesApply,
			Reply:         support.FirstNonEmpty(decision.Reply, "Please provide more detail so I can continue the aftersales request."),
			NeedHandoff:   decision.NeedHandoff,
			HandoffReason: decision.HandoffReason,
		}, nil
	default:
		return nil, subgraphcommon.InterruptForWriteAssist(ctx, decision)
	}
}

func buildAssistSlotsContext(currentOrder string, orderList []string, slotsText string) string {
	lines := make([]string, 0, 3)
	if currentOrder != "" {
		lines = append(lines, "current_order="+currentOrder)
	}
	if len(orderList) > 0 {
		lines = append(lines, "order_list="+strings.Join(orderList, ","))
	}
	if slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
