package productservice

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

// Build wires product-service read questions through the shared readflow.
func Build(ctx context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry, cacheLookup *globalnode.CacheLookupNode) (compose.AnyGraph, error) {
	return readflow.Build(ctx, readflow.Config{
		Intent:            domain.IntentProductService,
		AgentNodeName:     "ProductServiceAgentNode",
		ChatModel:         chatModel,
		Tools:             registry,
		Skills:            skills,
		ToolNames:         []string{"get_product", "get_inventory"},
		SystemHint:        agentPrompt,
		CacheLookup:       cacheLookup,
		BuildSlotsContext: buildSlotsContext,
		FallbackReply:     readflow.StaticFallback("Please tell me which product you want to check."),
	})
}

func buildSlotsContext(st *domain.State) string {
	session := st.Session
	lines := make([]string, 0, 3)
	if currentProduct := strings.TrimSpace(session.CurrentProduct); currentProduct != "" {
		lines = append(lines, "current_product="+currentProduct)
	}
	if currentSpec := strings.TrimSpace(session.CurrentSpec); currentSpec != "" {
		lines = append(lines, "current_spec="+currentSpec)
	}
	if len(session.ProductList) > 0 {
		lines = append(lines, "product_list="+strings.Join(session.ProductList, ","))
	}
	if slotsText := subgraphcommon.RenderSlotsContext(session.Slots); slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
