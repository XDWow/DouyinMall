package rewrite

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func Build(_ context.Context, chatModel model.ToolCallingChatModel, prompts *orchestratorprompt.Set, node *orchestratornode.RewriteNode) (compose.AnyGraph, error) {
	if chatModel == nil || prompts == nil || prompts.Rewrite == nil || node == nil {
		return nil, nil
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("RewriteEvaluateNode", compose.InvokableLambda(node.Evaluate), compose.WithNodeName("RewriteEvaluateNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("RewriteIdentityNode", compose.InvokableLambda(node.Identity), compose.WithNodeName("RewriteIdentityNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("BuildRewritePromptInputNode", compose.InvokableLambda(node.BuildPromptInput), compose.WithNodeName("BuildRewritePromptInputNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatTemplateNode("RewritePromptNode", prompts.Rewrite, compose.WithNodeName("RewritePromptNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatModelNode("RewriteModelNode", chatModel, compose.WithNodeName("RewriteModelNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyRewriteNode", compose.InvokableLambda(node.Apply), compose.WithNodeName("ApplyRewriteNode")); err != nil {
		return nil, err
	}
	if err := addEdge(g, compose.START, "RewriteEvaluateNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("RewriteEvaluateNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state != nil && state.Intent.NeedRewrite {
				return "BuildRewritePromptInputNode", nil
			}
			return "RewriteIdentityNode", nil
		},
		map[string]bool{"BuildRewritePromptInputNode": true, "RewriteIdentityNode": true},
	)); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{
		{"RewriteIdentityNode", compose.END},
		{"BuildRewritePromptInputNode", "RewritePromptNode"},
		{"RewritePromptNode", "RewriteModelNode"},
		{"RewriteModelNode", "ApplyRewriteNode"},
		{"ApplyRewriteNode", compose.END},
	} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	if err := g.AddEdge(start, end); err != nil {
		return fmt.Errorf("add edge %s -> %s: %w", start, end, err)
	}
	return nil
}
