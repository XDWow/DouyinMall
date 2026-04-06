package returnexchange

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// Input 描述退换货处理子图的入口。
type Input struct {
	Slots    map[string]any
	Message  string
	Intent   domain.Intent
	Recorder *agenttool.SafeExecutionRecorder
}

// Output 描述退换货处理子图的出口。
type Output struct {
	FinalAnswer     string
	NeedHandoff     bool
	HandoffReason   string
	ReadOnly        bool
	AwaitingConfirm bool
	ToolMessages    []*schema.Message
}

// Build 组装退换货处理子图。
// 这段流程会完成：订单预查询、资格校验、确认中断、提交售后。
func Build(
	_ context.Context,
	registry *agenttool.Registry,
	queryNode *orchestratornode.ReturnExchangeQueryNode,
	eligibilityNode *orchestratornode.EligibilityCheckNode,
	confirmNode *orchestratornode.ConfirmSummaryNode,
	submitNode *orchestratornode.SubmitAfterSaleNode,
) (compose.AnyGraph, error) {
	if queryNode == nil || eligibilityNode == nil || confirmNode == nil || submitNode == nil {
		return nil, nil
	}

	toolExecNode := orchestratornode.NewToolExecNode(registry)
	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ExecuteReturnExchangeFlowNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			slots := cloneSlots(input.Slots)
			if slots == nil {
				slots = map[string]any{}
			}
			needHandoff := false
			handoffReason := ""
			finalAnswer := ""
			readOnly := false
			awaitingConfirm := false

			queryResult, err := queryNode.Invoke(ctx, orchestratornode.ReturnExchangeQueryInput{Slots: slots})
			if err != nil {
				return Output{}, err
			}
			needHandoff = queryResult.NeedHandoff
			handoffReason = queryResult.HandoffReason
			finalAnswer = queryResult.FinalAnswer
			readOnly = queryResult.ReadOnly

			var toolMessages []*schema.Message
			if len(queryResult.Plans) > 0 && toolExecNode != nil {
				callMessage, callErr := toolexec.CreateToolCallMessage(queryResult.Plans)
				if callErr != nil {
					return Output{}, callErr
				}
				messages, execErr := toolExecNode.Invoke(ctx, orchestratornode.ToolExecutionInput{
					Plans:       queryResult.Plans,
					CallMessage: callMessage,
					Mode:        agenttool.ToolExecutionSerial,
				})
				if execErr != nil {
					return Output{}, execErr
				}
				toolMessages = append(toolMessages, messages...)
				if input.Recorder != nil {
					support.HydrateToolResultsIntoSlots(slots, input.Recorder.Snapshot())
				}
			}

			eligibilityResult, err := eligibilityNode.Invoke(ctx, orchestratornode.EligibilityCheckInput{
				Message:          input.Message,
				Slots:            slots,
				NeedHandoff:      needHandoff,
				AwaitingConfirm:  awaitingConfirm,
				QueryOrderResult: support.ToolResultRecordFromSlots(slots, "query_order"),
			})
			if err != nil {
				return Output{}, err
			}
			needHandoff = eligibilityResult.NeedHandoff
			handoffReason = eligibilityResult.HandoffReason
			if eligibilityResult.FinalAnswer != "" {
				finalAnswer = eligibilityResult.FinalAnswer
			}
			readOnly = eligibilityResult.ReadOnly
			awaitingConfirm = eligibilityResult.AwaitingConfirm
			confirmStatus := eligibilityResult.ConfirmStatus

			if awaitingConfirm {
				confirmResult, confirmErr := confirmNode.Invoke(ctx, orchestratornode.ConfirmSummaryInput{
					Reply:  finalAnswer,
					Intent: input.Intent,
				})
				if confirmErr != nil {
					return Output{}, confirmErr
				}
				if confirmResult != nil && confirmResult.Reply != "" {
					finalAnswer = confirmResult.Reply
				}
			}

			if strings.EqualFold(confirmStatus, "confirmed") && !needHandoff {
				plans, buildErr := buildAfterSaleSubmitPlan(ctx, registry, slots)
				if buildErr != nil {
					return Output{}, buildErr
				}
				if len(plans) == 0 {
					needHandoff = true
					handoffReason = "after_sale_service_unavailable"
					finalAnswer = "售后申请服务暂时不可用，已为你转人工处理。"
					return Output{
						FinalAnswer:     finalAnswer,
						NeedHandoff:     needHandoff,
						HandoffReason:   handoffReason,
						ReadOnly:        readOnly,
						AwaitingConfirm: false,
						ToolMessages:    append([]*schema.Message(nil), toolMessages...),
					}, nil
				}
				if len(plans) > 0 && toolExecNode != nil {
					callMessage, callErr := toolexec.CreateToolCallMessage(plans)
					if callErr != nil {
						return Output{}, callErr
					}
					messages, execErr := toolExecNode.Invoke(ctx, orchestratornode.ToolExecutionInput{
						Plans:       plans,
						CallMessage: callMessage,
						Mode:        agenttool.ToolExecutionSerial,
					})
					if execErr != nil {
						return Output{}, execErr
					}
					toolMessages = append(toolMessages, messages...)
					if input.Recorder != nil {
						support.HydrateToolResultsIntoSlots(slots, input.Recorder.Snapshot())
					}
				}
				submitResult, submitErr := submitNode.Invoke(ctx, orchestratornode.SubmitAfterSaleInput{
					ConfirmStatus: confirmStatus,
					RequestType:   support.FirstNonEmpty(slotString(slots, "request_type"), "return"),
					SubmitResult:  support.ToolResultMapFromSlots(slots, "create_after_sale_request"),
				})
				if submitErr != nil {
					return Output{}, submitErr
				}
				needHandoff = submitResult.NeedHandoff
				handoffReason = submitResult.HandoffReason
				if submitResult.FinalAnswer != "" {
					finalAnswer = submitResult.FinalAnswer
				}
				readOnly = submitResult.ReadOnly
				awaitingConfirm = submitResult.AwaitingConfirm
			}

			return Output{
				FinalAnswer:     finalAnswer,
				NeedHandoff:     needHandoff,
				HandoffReason:   handoffReason,
				ReadOnly:        readOnly,
				AwaitingConfirm: awaitingConfirm,
				ToolMessages:    append([]*schema.Message(nil), toolMessages...),
			}, nil
		}), compose.WithNodeName("ExecuteReturnExchangeFlowNode")); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "ExecuteReturnExchangeFlowNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ExecuteReturnExchangeFlowNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func buildAfterSaleSubmitPlan(ctx context.Context, registry *agenttool.Registry, slots map[string]any) ([]domain.ToolCallPlan, error) {
	if !registryHasTool(ctx, registry, "create_after_sale_request") {
		return nil, nil
	}
	orderID, err := parseSubmitOrderID(slots)
	if err != nil {
		return nil, err
	}
	args := map[string]any{
		"order_id":     orderID,
		"reason":       slotString(slots, "reason"),
		"request_type": support.FirstNonEmpty(slotString(slots, "request_type"), "return"),
	}
	if slotString(slots, "item_id", "sku_id", "product_id") != "" {
		if parsed, parseErr := parseSubmitItemID(slots); parseErr == nil {
			args["item_id"] = parsed
		}
	}
	return []domain.ToolCallPlan{{
		Name:      "create_after_sale_request",
		Arguments: args,
		Reason:    "submit_after_sale_request",
	}}, nil
}

func registryHasTool(ctx context.Context, registry *agenttool.Registry, name string) bool {
	if registry == nil {
		return false
	}
	for _, tool := range registry.Tools() {
		info, err := tool.Info(ctx)
		if err == nil && info != nil && info.Name == name {
			return true
		}
	}
	return false
}

func parseSubmitOrderID(slots map[string]any) (int64, error) {
	return strconv.ParseInt(slotString(slots, "order_id"), 10, 64)
}

func parseSubmitItemID(slots map[string]any) (int64, error) {
	return strconv.ParseInt(slotString(slots, "item_id", "sku_id", "product_id"), 10, 64)
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

func slotString(slots map[string]any, keys ...string) string {
	if len(slots) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := slots[key]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}
