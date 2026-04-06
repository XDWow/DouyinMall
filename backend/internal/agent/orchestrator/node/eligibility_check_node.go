package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// EligibilityCheckInput 描述售后资格校验真正需要的输入。
type EligibilityCheckInput struct {
	Message          string
	Slots            map[string]any
	NeedHandoff      bool
	AwaitingConfirm  bool
	QueryOrderResult map[string]any
}

// EligibilityCheckResult 描述售后资格校验后的结构化结果。
type EligibilityCheckResult struct {
	FinalAnswer     string
	NeedHandoff     bool
	HandoffReason   string
	ReadOnly        bool
	AwaitingConfirm bool
	ConfirmStatus   string
}

// EligibilityCheckNode 负责判断当前订单是否满足售后条件。
type EligibilityCheckNode struct{}

func NewEligibilityCheckNode() *EligibilityCheckNode { return &EligibilityCheckNode{} }

// Invoke 根据订单结果更新售后申请状态。
func (n *EligibilityCheckNode) Invoke(_ context.Context, input EligibilityCheckInput) (*EligibilityCheckResult, error) {
	result := &EligibilityCheckResult{
		NeedHandoff:     input.NeedHandoff,
		AwaitingConfirm: input.AwaitingConfirm,
	}
	if input.NeedHandoff {
		return result, nil
	}

	if input.AwaitingConfirm {
		switch {
		case support.IsNegative(input.Message):
			result.ConfirmStatus = "cancelled"
			result.AwaitingConfirm = false
			result.FinalAnswer = "已取消本次售后申请。"
		case support.IsAffirmative(input.Message):
			result.ConfirmStatus = "confirmed"
			result.AwaitingConfirm = false
		default:
			result.AwaitingConfirm = true
			result.FinalAnswer = support.BuildReturnApplySummaryFromSlots(input.Slots)
		}
		return result, nil
	}

	order := input.QueryOrderResult
	if len(order) == 0 {
		result.NeedHandoff = true
		result.HandoffReason = "order_not_found"
		result.FinalAnswer = "未找到对应订单，请确认订单号是否正确，或转人工继续处理。"
		return result, nil
	}

	if reply := buildAfterSaleIneligibleReply(input.Slots, order); reply != "" {
		result.AwaitingConfirm = false
		result.ReadOnly = true
		result.FinalAnswer = reply
		result.NeedHandoff = false
		result.HandoffReason = ""
		return result, nil
	}

	result.AwaitingConfirm = true
	result.FinalAnswer = support.BuildReturnApplySummaryFromSlots(input.Slots)
	return result, nil
}

func buildAfterSaleIneligibleReply(slots map[string]any, order map[string]any) string {
	if len(order) == 0 {
		return ""
	}

	reason := support.FirstNonEmpty(
		orderString(order, "after_sale_reason"),
		orderString(order, "ineligible_reason"),
		orderString(order, "reject_reason"),
		orderString(order, "reason"),
		orderString(order, "message"),
	)
	if ok, exists := support.ToolResultBool(order, "eligible", "after_sale_eligible", "can_after_sale", "can_apply_after_sale"); exists && !ok {
		return support.FirstNonEmpty(reason, "该订单当前不满足售后条件。")
	}
	if ok, exists := support.ToolResultBool(order, "in_after_sale_window", "within_after_sale_window"); exists && !ok {
		return support.FirstNonEmpty(reason, "该订单已超出售后时效。")
	}
	switch strings.ToLower(strings.TrimSpace(slotString(slots, "request_type"))) {
	case "exchange":
		if ok, exists := support.ToolResultBool(order, "support_exchange", "exchange_supported", "can_exchange"); exists && !ok {
			return support.FirstNonEmpty(reason, "该订单当前不支持换货。")
		}
	default:
		if ok, exists := support.ToolResultBool(order, "support_return", "return_supported", "can_return"); exists && !ok {
			return support.FirstNonEmpty(reason, "该订单当前不支持退货。")
		}
	}
	return ""
}

func orderString(order map[string]any, key string) string {
	value, ok := order[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
