package promotionservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type agentInput struct {
	UserMessage      string
	RewrittenQuery   string
	History          []*schema.Message
	CurrentPromotion string
	PromotionList    []string
	RetrievalResult  *ragnode.Result
	SlotsText        string
}

func Build(_ context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry, rag *ragnode.RAGNode) (compose.AnyGraph, error) {
	if chatModel == nil || rag == nil {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, 512)
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))

	wf.AddLambdaNode("PromotionRAGNode",
		compose.InvokableLambda(rag.Invoke),
		compose.WithStatePreHandler(func(_ context.Context, in ragnode.Input, st *domain.State) (ragnode.Input, error) {
			if st == nil || st.Input == nil || st.Session == nil {
				return in, fmt.Errorf("state input/session is required")
			}
			return ragnode.Input{
				Message: support.FirstNonEmpty(strings.TrimSpace(st.RewrittenQuery), strings.TrimSpace(st.Input.Message)),
				History: subgraphcommon.HistoryMessages(st.Session.RecentMessages),
				Intent:  string(domain.IntentPromotionService),
				Domains: []string{"platform"},
			}, nil
		}),
	).AddDependency(compose.START)

	wf.AddLambdaNode("PromotionServiceAgentNode",
		compose.InvokableLambda(func(ctx context.Context, in agentInput) (*domain.ChatResult, error) {
			finalText, _, err := agent.Run(ctx, sharednode.SubgraphAgentInput{
				DocumentsText: support.DocumentsText(documentsOf(in.RetrievalResult)),
				SlotsContext:  buildSlotsContext(in.CurrentPromotion, in.PromotionList, in.SlotsText),
				UserQuery:     support.FirstNonEmpty(in.RewrittenQuery, in.UserMessage),
				History:       in.History,
				SystemHint:    agentPrompt,
			})
			if err != nil {
				return nil, err
			}

			decision := subgraphcommon.ParseAgentDecision(finalText)
			if decision.Type == "clarification" {
				return nil, subgraphcommon.InterruptForDecision(ctx, decision)
			}
			return &domain.ChatResult{
				Intent:        domain.IntentPromotionService,
				Reply:         support.FirstNonEmpty(decision.Reply, support.BaseQAAnswerFromDocuments(documentsOf(in.RetrievalResult))),
				References:    support.DocumentsToReferences(documentsOf(in.RetrievalResult)),
				NeedHandoff:   decision.NeedHandoff,
				HandoffReason: decision.HandoffReason,
			}, nil
		}),
		compose.WithStatePreHandler(func(_ context.Context, in agentInput, st *domain.State) (agentInput, error) {
			if st == nil || st.Input == nil || st.Session == nil {
				return in, fmt.Errorf("state input/session is required")
			}
			in.UserMessage = strings.TrimSpace(st.Input.Message)
			in.RewrittenQuery = strings.TrimSpace(st.RewrittenQuery)
			in.History = subgraphcommon.HistoryMessages(st.Session.RecentMessages)
			in.CurrentPromotion = strings.TrimSpace(st.Session.CurrentPromotion)
			in.PromotionList = append([]string(nil), st.Session.PromotionList...)
			in.SlotsText = subgraphcommon.RenderSlotsContext(st.Session.Slots)
			return in, nil
		}),
	).AddInput("PromotionRAGNode", compose.ToField("RetrievalResult"))

	wf.End().AddInput("PromotionServiceAgentNode")
	return wf, nil
}

func documentsOf(result *ragnode.Result) []*schema.Document {
	if result == nil {
		return nil
	}
	return result.Documents
}

func buildSlotsContext(currentPromotion string, promotionList []string, slotsText string) string {
	lines := make([]string, 0, 2)
	if currentPromotion != "" {
		lines = append(lines, "current_promotion="+currentPromotion)
	}
	if len(promotionList) > 0 {
		lines = append(lines, "promotion_list="+strings.Join(promotionList, ","))
	}
	if slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
