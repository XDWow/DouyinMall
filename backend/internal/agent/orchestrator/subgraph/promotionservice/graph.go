package promotionservice

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

// Build wires promotion questions through cache -> RAG -> ADK agent.
func Build(ctx context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry, rag *ragnode.RAGNode, cacheLookup *globalnode.CacheLookupNode) (compose.AnyGraph, error) {
	return readflow.Build(ctx, readflow.Config{
		Intent:              domain.IntentPromotionService,
		AgentNodeName:       "PromotionServiceAgentNode",
		CacheLookupNodeName: "PromotionCacheLookupNode",
		CacheResultNodeName: "PromotionCacheResultNode",
		RAGNodeName:         "PromotionRAGNode",
		ChatModel:           chatModel,
		Tools:               registry,
		Skills:              skills,
		SystemHint:          agentPrompt,
		CacheLookup:         cacheLookup,
		RAG:                 rag,
		RAGDomains:          []string{"platform"},
		RequireRAG:          true,
		BuildSlotsContext:   buildSlotsContext,
		FallbackReply:       support.BaseQAAnswerFromDocuments,
		IncludeReferences:   true,
	})
}

func buildSlotsContext(st *domain.State) string {
	session := st.Session
	lines := make([]string, 0, 2)
	if currentPromotion := strings.TrimSpace(session.CurrentPromotion); currentPromotion != "" {
		lines = append(lines, "current_promotion="+currentPromotion)
	}
	if len(session.PromotionList) > 0 {
		lines = append(lines, "promotion_list="+strings.Join(session.PromotionList, ","))
	}
	if slotsText := subgraphcommon.RenderSlotsContext(session.Slots); slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
