package toolexec

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
)

// Input 描述工具执行子图真正需要的输入。
// 这里不再暴露主流程 State，而是只接收计划、执行模式和可选的调用消息。
type Input struct {
	Plans       []domain.ToolCallPlan
	CallMessage *schema.Message
	Mode        agenttool.ToolExecutionMode
}

// Output 描述工具执行子图的结果。
type Output struct {
	ToolMessages []*schema.Message
}

// Build 组装工具执行子图。
// 它本质上是一个 GraphNode，对外暴露明确的输入输出，内部只封装一次工具执行动作。
func Build(_ context.Context, registry *agenttool.Registry) (compose.AnyGraph, error) {
	if registry == nil {
		return nil, nil
	}

	execNode := sharednode.NewToolExecNode(registry)
	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ToolExecNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			messages, err := execNode.Invoke(ctx, sharednode.ToolExecutionInput{
				CallMessage: input.CallMessage,
				Plans:       input.Plans,
				Mode:        input.Mode,
			})
			if err != nil {
				return Output{}, err
			}
			return Output{ToolMessages: messages}, nil
		}), compose.WithNodeName("ToolExecNode")); err != nil {
		return nil, err
	}
	if err := addEdge(g, compose.START, "ToolExecNode"); err != nil {
		return nil, err
	}
	if err := addEdge(g, "ToolExecNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	return g.AddEdge(start, end)
}
