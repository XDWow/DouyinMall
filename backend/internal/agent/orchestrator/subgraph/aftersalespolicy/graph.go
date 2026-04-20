package aftersalespolicy

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
	UserMessage     string
	RewrittenQuery  string
	History         []*schema.Message
	CurrentOrder    string
	CurrentProduct  string
	CurrentSpec     string
	RetrievalResult *ragnode.Result
	SlotsText       string
}

func Build(_ context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry, rag *ragnode.RAGNode) (compose.AnyGraph, error) {
	if chatModel == nil || rag == nil {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, 512)
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))

	wf.AddLambdaNode("AftersalesPolicyRAGNode",
		compose.InvokableLambda(rag.Invoke),
		compose.WithStatePreHandler(func(_ context.Context, in ragnode.Input, st *domain.State) (ragnode.Input, error) {
			if st == nil || st.Input == nil || st.Session == nil {
				return in, fmt.Errorf("state input/session is required")
			}
			return ragnode.Input{
				Message: support.FirstNonEmpty(strings.TrimSpace(st.RewrittenQuery), strings.TrimSpace(st.Input.Message)),
				History: subgraphcommon.HistoryMessages(st.Session.RecentMessages),
				Intent:  string(domain.IntentAftersalesPolicy),
				Domains: []string{"aftersales", "platform"},
			}, nil
		}),
	).AddDependency(compose.START)

	wf.AddLambdaNode("AftersalesPolicyAgentNode",
		compose.InvokableLambda(func(ctx context.Context, in agentInput) (*domain.ChatResult, error) {
			finalText, _, err := agent.Run(ctx, sharednode.SubgraphAgentInput{
				ToolNames:     []string{"get_order", "list_user_orders", "query_order"},
				SkillNames:    []string{"return_policy_qa"},
				DocumentsText: support.DocumentsText(documentsOf(in.RetrievalResult)),
				SlotsContext:  buildSlotsContext(in.CurrentOrder, in.CurrentProduct, in.CurrentSpec, in.SlotsText),
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
			reply := support.FirstNonEmpty(decision.Reply, support.BaseQAAnswerFromDocuments(documentsOf(in.RetrievalResult)))
			return &domain.ChatResult{
				Intent:        domain.IntentAftersalesPolicy,
				Reply:         reply,
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
			in.CurrentOrder = strings.TrimSpace(st.Session.CurrentOrder)
			in.CurrentProduct = strings.TrimSpace(st.Session.CurrentProduct)
			in.CurrentSpec = strings.TrimSpace(st.Session.CurrentSpec)
			in.SlotsText = subgraphcommon.RenderSlotsContext(st.Session.Slots)
			return in, nil
		}),
	).AddInput("AftersalesPolicyRAGNode", compose.ToField("RetrievalResult"))

	wf.End().AddInput("AftersalesPolicyAgentNode")
	return wf, nil
}

func documentsOf(result *ragnode.Result) []*schema.Document {
	if result == nil {
		return nil
	}
	return result.Documents
}

func buildSlotsContext(currentOrder, currentProduct, currentSpec, slotsText string) string {
	lines := make([]string, 0, 4)
	if currentOrder != "" {
		lines = append(lines, "current_order="+currentOrder)
	}
	if currentProduct != "" {
		lines = append(lines, "current_product="+currentProduct)
	}
	if currentSpec != "" {
		lines = append(lines, "current_spec="+currentSpec)
	}
	if slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
