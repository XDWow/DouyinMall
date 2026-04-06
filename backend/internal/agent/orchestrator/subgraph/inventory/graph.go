package inventory

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
)

// Input 描述库存查询子图的入口。
type Input struct {
	Slots map[string]any
}

// Output 描述库存查询子图的出口。
type Output struct {
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
}

// Build 组装库存查询子图。
func Build(ctx context.Context, registry *agenttool.Registry, node *orchestratornode.InventoryReadNode) (compose.AnyGraph, error) {
	if node == nil {
		return nil, nil
	}

	_ = ctx
	toolExecNode := orchestratornode.NewToolExecNode(registry)

	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ExecuteInventoryFlowNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			result, err := node.Invoke(ctx, orchestratornode.InventoryReadInput{Slots: cloneSlots(input.Slots)})
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
			messages, err := toolExecNode.Invoke(ctx, orchestratornode.ToolExecutionInput{
				Plans:       result.Plans,
				CallMessage: callMessage,
				Mode:        agenttool.ToolExecutionSerial,
			})
			if err != nil {
				return Output{}, err
			}
			out.ToolMessages = append([]*schema.Message(nil), messages...)
			return out, nil
		}), compose.WithNodeName("ExecuteInventoryFlowNode")); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "ExecuteInventoryFlowNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ExecuteInventoryFlowNode", compose.END); err != nil {
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
