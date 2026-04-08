package global

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	orchestratorshared "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
)

type GlobalSlotCheckInput struct {
	Intent          domain.Intent
	Slots           map[string]any
	RawQuery        string
	AwaitingConfirm bool
	NeedHandoff     bool
}

type GlobalSlotCheckNode struct{}

func NewGlobalSlotCheckNode() *GlobalSlotCheckNode { return &GlobalSlotCheckNode{} }

type GlobalSlotCheckResult struct {
	MissingSlots []string
	AwaitingUser bool
	FinalAnswer  string
}

func (n *GlobalSlotCheckNode) Invoke(_ context.Context, input GlobalSlotCheckInput) (*GlobalSlotCheckResult, error) {
	missingSlots := requiredMissingSlots(input.Intent, input.Slots, input.AwaitingConfirm)
	awaitingUser := len(missingSlots) > 0
	finalAnswer := ""
	if awaitingUser {
		finalAnswer = askMessageForMissingSlot(input.Intent, missingSlots[0])
	}

	return &GlobalSlotCheckResult{
		MissingSlots: missingSlots,
		AwaitingUser: awaitingUser,
		FinalAnswer:  finalAnswer,
	}, nil
}

func (n *GlobalSlotCheckNode) Apply(ctx context.Context, state *graphstate.State) (*graphstate.State, error) {
	if state == nil {
		return nil, nil
	}

	result, err := n.Invoke(ctx, GlobalSlotCheckInput{
		Intent:          state.Session.Intent,
		Slots:           state.Session.Slots,
		RawQuery:        state.Session.RawQuery,
		AwaitingConfirm: state.Session.AwaitingConfirm,
		NeedHandoff:     state.Session.NeedHandoff,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return state, nil
	}

	state.Session.MissingSlots = result.MissingSlots
	state.Session.AwaitingUser = result.AwaitingUser
	if result.FinalAnswer != "" {
		state.Session.FinalAnswer = result.FinalAnswer
	}
	return state, nil
}

func requiredMissingSlots(intent domain.Intent, slots map[string]any, awaitingConfirm bool) []string {
	switch intent {
	case domain.IntentInventoryQuery, domain.IntentProductInfo, domain.IntentAddToCart:
		if strings.TrimSpace(orchestratorshared.SlotString(slots, "product_id")) == "" {
			return []string{"product_id"}
		}
	case domain.IntentReturnExchangeApply:
		if awaitingConfirm {
			return nil
		}
		if strings.TrimSpace(orchestratorshared.SlotString(slots, "order_id")) == "" {
			return []string{"order_id"}
		}
		if strings.TrimSpace(orchestratorshared.SlotString(slots, "reason")) == "" {
			return []string{"reason"}
		}
	}
	return nil
}

func askMessageForMissingSlot(intent domain.Intent, slot string) string {
	switch slot {
	case "order_id":
		if intent == domain.IntentReturnExchangeApply {
			return "\u8bf7\u5148\u63d0\u4f9b\u8ba2\u5355\u53f7\uff0c\u6211\u518d\u7ee7\u7eed\u4e3a\u4f60\u5904\u7406\u552e\u540e\u7533\u8bf7\u3002"
		}
		return "\u8bf7\u63d0\u4f9b\u8ba2\u5355\u53f7\uff0c\u6211\u518d\u7ee7\u7eed\u4e3a\u4f60\u5904\u7406\u3002"
	case "product_id":
		switch intent {
		case domain.IntentInventoryQuery:
			return "\u8bf7\u63d0\u4f9b\u5546\u54c1 ID \u6216 SKU\uff0c\u6211\u6765\u5e2e\u4f60\u67e5\u5e93\u5b58\u3002"
		case domain.IntentAddToCart:
			return "\u8bf7\u544a\u8bc9\u6211\u4f60\u60f3\u52a0\u5165\u8d2d\u7269\u8f66\u7684\u5546\u54c1 ID\u3002"
		default:
			return "\u8bf7\u63d0\u4f9b\u5546\u54c1 ID \u6216 SKU\uff0c\u6211\u518d\u7ee7\u7eed\u4e3a\u4f60\u5904\u7406\u3002"
		}
	case "reason":
		return "\u8bf7\u544a\u8bc9\u6211\u552e\u540e\u539f\u56e0\uff0c\u6bd4\u5982\u5546\u54c1\u7834\u635f\u3001\u5c3a\u7801\u4e0d\u5408\u9002\u6216\u4e0d\u60f3\u8981\u4e86\u3002"
	default:
		return "\u8bf7\u5148\u8865\u5145\u7f3a\u5931\u4fe1\u606f\uff0c\u6211\u518d\u7ee7\u7eed\u4e3a\u4f60\u5904\u7406\u3002"
	}
}
