package productservice

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
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type agentInput struct {
	UserMessage    string
	RewrittenQuery string
	History        []*schema.Message
	CurrentProduct string
	CurrentSpec    string
	ProductList    []string
	SlotsText      string
}

func Build(_ context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry) (compose.AnyGraph, error) {
	if chatModel == nil {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, 512)
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))
	wf.AddLambdaNode("ProductServiceAgentNode",
		compose.InvokableLambda(func(ctx context.Context, in agentInput) (*domain.ChatResult, error) {
			finalText, _, err := agent.Run(ctx, sharednode.SubgraphAgentInput{
				ToolNames:    []string{"get_product", "get_inventory"},
				SlotsContext: buildSlotsContext(in.CurrentProduct, in.CurrentSpec, in.ProductList, in.SlotsText),
				UserQuery:    support.FirstNonEmpty(in.RewrittenQuery, in.UserMessage),
				History:      in.History,
				SystemHint:   agentPrompt,
			})
			if err != nil {
				return nil, err
			}

			decision := subgraphcommon.ParseAgentDecision(finalText)
			if decision.Type == "clarification" {
				return nil, subgraphcommon.InterruptForDecision(ctx, decision)
			}
			return &domain.ChatResult{
				Intent:        domain.IntentProductService,
				Reply:         support.FirstNonEmpty(decision.Reply, "Please tell me which product you want to check."),
				NeedHandoff:   decision.NeedHandoff,
				HandoffReason: decision.HandoffReason,
			}, nil
		}),
		compose.WithStatePreHandler(func(_ context.Context, in agentInput, st *domain.State) (agentInput, error) {
			if st == nil || st.Input == nil || st.Session == nil {
				return in, fmt.Errorf("state input/session is required")
			}
			return agentInput{
				UserMessage:    strings.TrimSpace(st.Input.Message),
				RewrittenQuery: strings.TrimSpace(st.RewrittenQuery),
				History:        subgraphcommon.HistoryMessages(st.Session.RecentMessages),
				CurrentProduct: strings.TrimSpace(st.Session.CurrentProduct),
				CurrentSpec:    strings.TrimSpace(st.Session.CurrentSpec),
				ProductList:    append([]string(nil), st.Session.ProductList...),
				SlotsText:      subgraphcommon.RenderSlotsContext(st.Session.Slots),
			}, nil
		}),
	).AddDependency(compose.START)
	wf.End().AddInput("ProductServiceAgentNode")
	return wf, nil
}

func buildSlotsContext(currentProduct, currentSpec string, productList []string, slotsText string) string {
	lines := make([]string, 0, 3)
	if currentProduct != "" {
		lines = append(lines, "current_product="+currentProduct)
	}
	if currentSpec != "" {
		lines = append(lines, "current_spec="+currentSpec)
	}
	if len(productList) > 0 {
		lines = append(lines, "product_list="+strings.Join(productList, ","))
	}
	if slotsText != "" {
		lines = append(lines, slotsText)
	}
	return strings.Join(lines, "\n")
}
