package orderquery

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	ordernode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/order"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
)

// Input 描述订单查询子图的入口。
type Input struct {
	Slots map[string]any
}

// Output 描述订单查询子图的出口。
type Output struct {
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
}

// Build 组装订单查询子图。
// 这段流程的职责是：生成订单查询计划，按需执行工具，再把结果整理成明确输出。
func Build(ctx context.Context, registry *agenttool.Registry, node *ordernode.OrderReadNode) (compose.AnyGraph, error) {
	if node == nil {
		return nil, nil
	}

	_ = ctx
	toolExecNode := sharednode.NewToolExecNode(registry)

	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ExecuteOrderQueryFlowNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			result, err := node.Invoke(ctx, ordernode.OrderReadInput{Slots: cloneSlots(input.Slots)})
			if err != nil {
				return Output{}, err
			}

			out := Output{
				FinalAnswer:   result.FinalAnswer,
				NeedHandoff:   result.NeedHandoff,
				HandoffReason: result.HandoffReason,
				ReadOnly:      result.ReadOnly,
			}
			if len(result.Plans) == 0 || toolExecNode == nil {
				return out, nil
			}

			callMessage, err := toolexec.CreateToolCallMessage(result.Plans)
			if err != nil {
				return Output{}, err
			}
			messages, err := toolExecNode.Invoke(ctx, sharednode.ToolExecutionInput{
				Plans:       result.Plans,
				CallMessage: callMessage,
				Mode:        agenttool.ToolExecutionSerial,
			})
			if err != nil {
				return Output{}, err
			}
			out.ToolMessages = append([]*schema.Message(nil), messages...)
			return out, nil
		}), compose.WithNodeName("ExecuteOrderQueryFlowNode")); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "ExecuteOrderQueryFlowNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ExecuteOrderQueryFlowNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneSlots(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
