package aftersalespolicy

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/readflow"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// Build wires aftersales-policy questions through cache -> RAG -> ADK agent.
func Build(ctx context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry, rag *ragnode.RAGNode, cacheLookup *globalnode.CacheLookupNode) (compose.AnyGraph, error) {
	return readflow.Build(ctx, readflow.Config{
		Intent:              domain.IntentAftersalesPolicy,
		AgentNodeName:       "AftersalesPolicyAgentNode",
		CacheLookupNodeName: "AftersalesPolicyCacheLookupNode",
		CacheResultNodeName: "AftersalesPolicyCacheResultNode",
		RAGNodeName:         "AftersalesPolicyRAGNode",
		ChatModel:           chatModel,
		Tools:               registry,
		Skills:              skills,
		ToolNames:           []string{"get_order", "list_user_orders", "query_order"},
		SkillNames:          []string{"return_policy_qa"},
		SystemHint:          agentPrompt,
		CacheLookup:         cacheLookup,
		RAG:                 rag,
		RAGDomains:          []string{"aftersales", "platform"},
		RequireRAG:          true,
		BuildSlotsContext:   buildSlotsContext,
		FallbackReply:       support.BaseQAAnswerFromDocuments,
		IncludeReferences:   true,
	})
}

func buildSlotsContext(st *domain.State) string {
	session := st.Session
	lines := make([]string, 0, 4)
	if currentOrder := strings.TrimSpace(session.CurrentOrder); currentOrder != "" {
		lines = append(lines, "current_order="+currentOrder)
	}
	if currentProduct := strings.TrimSpace(session.CurrentProduct); currentProduct != "" {
		lines = append(lines, "current_product="+currentProduct)
	}
	if currentSpec := strings.TrimSpace(session.CurrentSpec); currentSpec != "" {
		lines = append(lines, "current_spec="+currentSpec)
	}
	if slotsText := subgraphcommon.RenderSlotsContext(session.Slots); slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
