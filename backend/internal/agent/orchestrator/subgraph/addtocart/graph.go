package addtocart

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

func Build(_ context.Context, chatModel model.ToolCallingChatModel, registry *agenttool.Registry, skills *agentskill.Registry) (compose.AnyGraph, error) {
	wf := compose.NewWorkflow[struct{}, *domain.ChatResult](compose.WithGenLocalState(domain.SharedGraphState))
	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, 384)

	wf.AddLambdaNode("AddToCartAssistNode",
		compose.InvokableLambda(func(ctx context.Context, in assistInput) (*domain.ChatResult, error) {
			return runAgentAssist(ctx, agent, in)
		}),
		compose.WithStatePreHandler(func(_ context.Context, in assistInput, st *domain.State) (assistInput, error) {
			return assistInputFromState(st)
		}),
	).AddDependency(compose.START)

	wf.AddLambdaNode("AddToCartResolveNode",
		compose.InvokableLambda(resolveInvoke),
		compose.WithStatePreHandler(func(_ context.Context, in AddToCartResolveInput, st *domain.State) (AddToCartResolveInput, error) {
			return InputFromState(st)
		}),
	).AddDependency("AddToCartAssistNode")

	wf.AddLambdaNode("AddToCartAssistResultNode", compose.InvokableLambda(
		func(_ context.Context, in *domain.ChatResult) (*domain.ChatResult, error) {
			return in, nil
		},
	)).AddInput("AddToCartAssistNode")

	wf.AddLambdaNode("AddToCartEnsureArgsNode", compose.InvokableLambda(ensureAddToCartArgs)).
		AddInput("AddToCartResolveNode")

	wf.AddLambdaNode("AddToCartSubmitNode", compose.InvokableLambda(
		func(ctx context.Context, resolved ResolvedAddToCart) (*domain.ChatResult, error) {
			return submitAddToCart(ctx, resolved, registry)
		},
	)).AddInput("AddToCartEnsureArgsNode")

	wf.AddBranch("AddToCartAssistNode", compose.NewGraphBranch(
		func(_ context.Context, in *domain.ChatResult) (string, error) {
			if in != nil {
				return "AddToCartAssistResultNode", nil
			}
			return "AddToCartResolveNode", nil
		},
		map[string]bool{
			"AddToCartResolveNode":      true,
			"AddToCartAssistResultNode": true,
		},
	))

	wf.End().
		AddInput("AddToCartSubmitNode").
		AddInput("AddToCartAssistResultNode")
	return wf, nil
}
