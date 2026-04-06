package node

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// SlotCheckInput 描述槽位校验阶段的输入。
type SlotCheckInput struct {
	Intent          domain.Intent
	Slots           map[string]any
	RawQuery        string
	AwaitingConfirm bool
	NeedHandoff     bool
}

// SlotCheckNode 负责判断当前还缺哪些槽位，以及是否需要继续追问用户。
type SlotCheckNode struct{}

func NewSlotCheckNode() *SlotCheckNode { return &SlotCheckNode{} }

type SlotCheckResult struct {
	MissingSlots []string
	AwaitingUser bool
	FinalAnswer  string
}

// Invoke 返回槽位校验结果。
func (n *SlotCheckNode) Invoke(_ context.Context, input SlotCheckInput) (*SlotCheckResult, error) {
	missingSlots := requiredMissingSlots(input.Intent, input.Slots, input.AwaitingConfirm)
	awaitingUser := len(missingSlots) > 0
	finalAnswer := ""
	if awaitingUser {
		finalAnswer = askMessageForMissingSlot(input.Intent, missingSlots[0])
	}

	return &SlotCheckResult{
		MissingSlots: missingSlots,
		AwaitingUser: awaitingUser,
		FinalAnswer:  finalAnswer,
	}, nil
}

func requiredMissingSlots(intent domain.Intent, slots map[string]any, awaitingConfirm bool) []string {
	switch intent {
	case domain.IntentInventoryQuery, domain.IntentProductInfo, domain.IntentAddToCart:
		if strings.TrimSpace(slotString(slots, "product_id")) == "" {
			return []string{"product_id"}
		}
	case domain.IntentReturnExchangeApply:
		if awaitingConfirm {
			return nil
		}
		if strings.TrimSpace(slotString(slots, "order_id")) == "" {
			return []string{"order_id"}
		}
		if strings.TrimSpace(slotString(slots, "reason")) == "" {
			return []string{"reason"}
		}
	}
	return nil
}

func askMessageForMissingSlot(intent domain.Intent, slot string) string {
	switch slot {
	case "order_id":
		if intent == domain.IntentReturnExchangeApply {
			return "请先提供订单号，我再继续为你处理售后申请。"
		}
		return "请提供订单号，我再继续为你处理。"
	case "product_id":
		switch intent {
		case domain.IntentInventoryQuery:
			return "请提供商品 ID 或 SKU，我来帮你查询库存。"
		case domain.IntentAddToCart:
			return "请告诉我你想加入购物车的商品 ID。"
		default:
			return "请提供商品 ID 或 SKU，我再继续为你处理。"
		}
	case "reason":
		return "请告诉我售后原因，例如商品破损、尺码不合适或不想要了。"
	default:
		return "请先补充缺失信息，我再继续为你处理。"
	}
}
