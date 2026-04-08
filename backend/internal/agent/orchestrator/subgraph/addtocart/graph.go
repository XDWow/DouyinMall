package addtocart

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	cartnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/cart"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// Input 描述加购子图的入口。
type Input struct {
	Slots    map[string]any
	Recorder *agenttool.SafeExecutionRecorder
}

// Output 描述加购子图的出口。
type Output struct {
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
}

// Build 组装加购子图。
// 这段流程会先生成加购计划，再执行工具，并根据工具结果补出最终回复。
func Build(ctx context.Context, registry *agenttool.Registry, node *cartnode.AddToCartNode) (compose.AnyGraph, error) {
	if node == nil {
		return nil, nil
	}

	_ = ctx
	toolExecNode := sharednode.NewToolExecNode(registry)

	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ExecuteAddToCartFlowNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			slots := cloneSlots(input.Slots)
			if slots == nil {
				slots = map[string]any{}
			}

			result, err := node.Invoke(ctx, cartnode.AddToCartInput{Slots: slots})
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

			if input.Recorder != nil {
				support.HydrateToolResultsIntoSlots(slots, input.Recorder.Snapshot())
			}
			if record := support.ToolResultRecordFromSlots(slots, "add_to_cart"); len(record) > 0 {
				if ok, exists := support.ToolResultBool(record, "success"); exists && ok {
					productID := support.FirstNonEmpty(fmt.Sprint(slots["product_id"]), "unknown")
					quantity := int64(1)
					if raw := fmt.Sprint(slots["quantity"]); raw != "" && raw != "<nil>" {
						if q, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil && q > 0 {
							quantity = q
						}
					}
					out.FinalAnswer = fmt.Sprintf("\u5546\u54c1 %s \u5df2\u52a0\u5165\u8d2d\u7269\u8f66\uff0c\u6570\u91cf %d\u3002", productID, quantity)
				}
			}
			return out, nil
		}), compose.WithNodeName("ExecuteAddToCartFlowNode")); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "ExecuteAddToCartFlowNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ExecuteAddToCartFlowNode", compose.END); err != nil {
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
