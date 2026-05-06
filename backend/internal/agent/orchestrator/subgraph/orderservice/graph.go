package orderservice

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/readflow"
)

// Build wires order-service read questions through the shared readflow.
func Build(ctx context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry, cacheLookup *globalnode.CacheLookupNode) (compose.AnyGraph, error) {
	return readflow.Build(ctx, readflow.Config{
		Intent:            domain.IntentOrderService,
		AgentNodeName:     "OrderServiceAgentNode",
		ChatModel:         chatModel,
		Tools:             registry,
		Skills:            skills,
		ToolNames:         []string{"get_order", "list_user_orders", "query_order"},
		SkillNames:        []string{"order_lookup"},
		SystemHint:        agentPrompt,
		CacheLookup:       cacheLookup,
		BuildSlotsContext: buildSlotsContext,
		FallbackReply:     readflow.StaticFallback("Please tell me which order you want to check."),
	})
}

func buildSlotsContext(st *domain.State) string {
	session := st.Session
	lines := make([]string, 0, 2)
	if currentOrder := strings.TrimSpace(session.CurrentOrder); currentOrder != "" {
		lines = append(lines, "current_order="+currentOrder)
	}
	if len(session.OrderList) > 0 {
		lines = append(lines, "order_list="+strings.Join(session.OrderList, ","))
	}
	if slotsText := subgraphcommon.RenderSlotsContext(session.Slots); slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
