package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/graph/node"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/graph/prompt"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/tool"
	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
)

// Builder creates a small set of reusable subgraphs. The main orchestration
// still lives in graph/app, while this package only wraps repeated business
// paths such as order query or return policy.
type Builder struct {
	Model     model.ToolCallingChatModel
	Retriever einoretriever.Retriever
	Registry  *agenttool.Registry
	Prompts   *orchestratorprompt.Set
	Nodes     *orchestratornode.Suite
}

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	if err := g.AddEdge(start, end); err != nil {
		return fmt.Errorf("add edge %s -> %s: %w", start, end, err)
	}
	return nil
}

func CreateToolDecisionMessage(plans []dto.ToolCallPlan) (*schema.Message, error) {
	if len(plans) == 0 {
		return nil, nil
	}
	toolCalls := make([]schema.ToolCall, 0, len(plans))
	for _, plan := range plans {
		rawJSON := strings.TrimSpace(plan.RawJSON)
		if rawJSON == "" {
			payload, err := json.Marshal(plan.Arguments)
			if err != nil {
				return nil, err
			}
			rawJSON = string(payload)
		}
		toolCalls = append(toolCalls, schema.ToolCall{
			ID:   "call_" + uuid.NewString(),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      plan.Name,
				Arguments: rawJSON,
			},
		})
	}
	return schema.AssistantMessage("", toolCalls), nil
}

func (b *Builder) BuildIntentClassificationChain() (compose.AnyGraph, error) {
	if b.Model == nil || b.Prompts == nil || b.Prompts.Intent == nil || b.Nodes == nil {
		return nil, nil
	}
	intent := b.Nodes.IntentClassify()
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode("BuildIntentPromptInputNode", compose.InvokableLambda(intent.BuildPromptInput), compose.WithNodeName("BuildIntentPromptInputNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatTemplateNode("IntentPromptNode", b.Prompts.Intent, compose.WithNodeName("IntentPromptNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatModelNode("IntentModelNode", b.Model, compose.WithNodeName("IntentModelNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyIntentNode", compose.InvokableLambda(intent.Apply), compose.WithNodeName("ApplyIntentNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{{compose.START, "BuildIntentPromptInputNode"}, {"BuildIntentPromptInputNode", "IntentPromptNode"}, {"IntentPromptNode", "IntentModelNode"}, {"IntentModelNode", "ApplyIntentNode"}, {"ApplyIntentNode", compose.END}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (b *Builder) BuildRewriteChain() (compose.AnyGraph, error) {
	if b.Model == nil || b.Prompts == nil || b.Prompts.Rewrite == nil || b.Nodes == nil {
		return nil, nil
	}
	rewrite := b.Nodes.Rewrite()
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode("RewriteEvaluateNode", compose.InvokableLambda(rewrite.Evaluate), compose.WithNodeName("RewriteEvaluateNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("RewriteIdentityNode", compose.InvokableLambda(rewrite.Identity), compose.WithNodeName("RewriteIdentityNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("BuildRewritePromptInputNode", compose.InvokableLambda(rewrite.BuildPromptInput), compose.WithNodeName("BuildRewritePromptInputNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatTemplateNode("RewritePromptNode", b.Prompts.Rewrite, compose.WithNodeName("RewritePromptNode")); err != nil {
		return nil, err
	}
	if err := g.AddChatModelNode("RewriteModelNode", b.Model, compose.WithNodeName("RewriteModelNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyRewriteNode", compose.InvokableLambda(rewrite.Apply), compose.WithNodeName("ApplyRewriteNode")); err != nil {
		return nil, err
	}
	if err := addEdge(g, compose.START, "RewriteEvaluateNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("RewriteEvaluateNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.FlowContext) (string, error) {
			flow := orchestratorstate.ConversationFlowFromContext(ctx)
			if flow != nil && flow.Intent.NeedRewrite {
				return "BuildRewritePromptInputNode", nil
			}
			return "RewriteIdentityNode", nil
		},
		map[string]bool{"BuildRewritePromptInputNode": true, "RewriteIdentityNode": true},
	)); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{{"RewriteIdentityNode", compose.END}, {"BuildRewritePromptInputNode", "RewritePromptNode"}, {"RewritePromptNode", "RewriteModelNode"}, {"RewriteModelNode", "ApplyRewriteNode"}, {"ApplyRewriteNode", compose.END}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (b *Builder) BuildRetrieveChain() (compose.AnyGraph, error) {
	if b.Retriever == nil || b.Nodes == nil {
		return nil, nil
	}
	retrieve := b.Nodes.Retrieve()
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode("PrepareRetrieveQueryNode", compose.InvokableLambda(retrieve.PrepareQuery), compose.WithNodeName("PrepareRetrieveQueryNode")); err != nil {
		return nil, err
	}
	if err := g.AddRetrieverNode("RetrieverNode", b.Retriever, compose.WithNodeName("RetrieverNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyRetrieveNode", compose.InvokableLambda(retrieve.ApplyDocuments), compose.WithNodeName("ApplyRetrieveNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{{compose.START, "PrepareRetrieveQueryNode"}, {"PrepareRetrieveQueryNode", "RetrieverNode"}, {"RetrieverNode", "ApplyRetrieveNode"}, {"ApplyRetrieveNode", compose.END}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (b *Builder) BuildToolExecWorkflow(mode agenttool.ToolExecutionMode) (compose.AnyGraph, error) {
	if b.Registry == nil || b.Nodes == nil {
		return nil, nil
	}
	toolsNode, err := b.Registry.ToolsNode(mode)
	if err != nil {
		return nil, err
	}
	toolNode := b.Nodes.ToolExec()
	prepareName := "PrepareSerialToolMessageNode"
	prepareFn := toolNode.PrepareSerialMessage
	if mode == agenttool.ToolExecutionParallelReadOnly {
		prepareName = "PrepareParallelReadonlyToolMessageNode"
		prepareFn = toolNode.PrepareParallelReadOnlyMessage
	}
	g := compose.NewGraph[*orchestratorstate.FlowContext, *orchestratorstate.FlowContext]()
	if err := g.AddLambdaNode(prepareName, compose.InvokableLambda(prepareFn), compose.WithNodeName(prepareName)); err != nil {
		return nil, err
	}
	if err := g.AddToolsNode("ToolsNode", toolsNode, compose.WithNodeName("ToolsNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ApplyToolMessagesNode", compose.InvokableLambda(toolNode.ApplyMessages), compose.WithNodeName("ApplyToolMessagesNode")); err != nil {
		return nil, err
	}
	for _, edge := range [][2]string{{compose.START, prepareName}, {prepareName, "ToolsNode"}, {"ToolsNode", "ApplyToolMessagesNode"}, {"ApplyToolMessagesNode", compose.END}} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}
	return g, nil
}
