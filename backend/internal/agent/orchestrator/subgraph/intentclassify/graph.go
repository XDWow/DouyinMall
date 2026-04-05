package intentclassify

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

func Build(_ context.Context, chatModel model.ToolCallingChatModel, prompts *orchestratorprompt.Set, node *orchestratornode.IntentClassifyNode) (compose.AnyGraph, error) {
	if chatModel == nil || prompts == nil || prompts.Intent == nil || node == nil {
		return nil, nil
	}
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("BuildIntentPromptInputNode", compose.InvokableLambda(node.BuildPromptInput), compose.WithNodeName("BuildIntentPromptInputNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatTemplateNode("IntentPromptNode", prompts.Intent, compose.WithNodeName("IntentPromptNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatModelNode("IntentModelNode", chatModel, compose.WithNodeName("IntentModelNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyIntentNode", compose.InvokableLambda(node.Apply), compose.WithNodeName("ApplyIntentNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{
		{compose.START, "BuildIntentPromptInputNode"},
		{"BuildIntentPromptInputNode", "IntentPromptNode"},
		{"IntentPromptNode", "IntentModelNode"},
		{"IntentModelNode", "ApplyIntentNode"},
		{"ApplyIntentNode", compose.END},
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
