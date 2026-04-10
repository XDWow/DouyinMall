package returnexchange

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	aftersalenode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/aftersale"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// reState 退换货子图内部状态（多节点串联）。
type reState struct {
	Slots           map[string]any
	Message         string
	Intent          domain.Intent
	AwaitingConfirm bool
	Recorder        domain.ToolExecutionSink
	MissingSlots    []string
	NeedHandoff     bool
	HandoffReason   string
	FinalAnswer     string
	ReadOnly        bool
	ToolMessages    []*schema.Message
	ConfirmStatus   string
}

func reInitSlots(ctx context.Context, _ struct{}) (reState, error) {
	var s reState
	if err := domain.ProcessState(ctx, func(st *domain.State) error {
		if st == nil {
			return fmt.Errorf("state is nil")
		}
		s.Message = st.Input.Message
		s.Intent = st.Session.Intent
		s.AwaitingConfirm = st.Session.AwaitingConfirm
		s.Recorder = st.Recorder
		slots := cloneSlotsRE(st.Session.Slots)
		if slots == nil {
			slots = map[string]any{}
		}
		globalnode.ApplyIntentFieldsForTools(slots, st.Intent.Entities)
		s.Slots = slots
		s.MissingSlots = globalnode.RequiredMissingSlots(domain.IntentReturnExchangeApply, slots, st.Intent.Entities, s.AwaitingConfirm)
		return nil
	}); err != nil {
		return reState{}, err
	}
	return s, nil
}

func branchAfterRESlotCheck(_ context.Context, s reState) (string, error) {
	if len(s.MissingSlots) > 0 {
		return "ReturnExchangeMissingSlotsNode", nil
	}
	return "ReturnExchangeQueryNode", nil
}

func reBuildMissingOutput(_ context.Context, s reState) (Output, error) {
	m := s.MissingSlots[0]
	return Output{
		FinalAnswer:  globalnode.AskMessageForMissingSlot(domain.IntentReturnExchangeApply, m),
		ReadOnly:     true,
		AwaitingUser: true,
		MissingSlots: append([]string(nil), s.MissingSlots...),
	}, nil
}

func reRunQuery(queryNode *aftersalenode.ReturnExchangeQueryNode, toolExec *sharednode.ToolExecNode) func(context.Context, reState) (reState, error) {
	return func(ctx context.Context, s reState) (reState, error) {
		queryResult, err := queryNode.Invoke(ctx, aftersalenode.ReturnExchangeQueryInput{Slots: s.Slots})
		if err != nil {
			return reState{}, err
		}
		s.NeedHandoff = queryResult.NeedHandoff
		s.HandoffReason = queryResult.HandoffReason
		s.FinalAnswer = queryResult.FinalAnswer
		s.ReadOnly = queryResult.ReadOnly
		if len(queryResult.Plans) > 0 && toolExec != nil {
			callMessage, callErr := toolexec.CreateToolCallMessage(queryResult.Plans)
			if callErr != nil {
				return reState{}, callErr
			}
			messages, execErr := toolExec.Invoke(ctx, sharednode.ToolExecutionInput{
				Plans:       queryResult.Plans,
				CallMessage: callMessage,
				Mode:        agenttool.ToolExecutionSerial,
			})
			if execErr != nil {
				return reState{}, execErr
			}
			s.ToolMessages = append(s.ToolMessages, messages...)
			if s.Recorder != nil {
				support.HydrateToolResultsIntoSlots(s.Slots, s.Recorder.Snapshot())
			}
		}
		return s, nil
	}
}

func reRunEligibility(eligibilityNode *aftersalenode.EligibilityCheckNode) func(context.Context, reState) (reState, error) {
	return func(ctx context.Context, s reState) (reState, error) {
		awaitingConfirm := false
		eligibilityResult, err := eligibilityNode.Invoke(ctx, aftersalenode.EligibilityCheckInput{
			Message:          s.Message,
			Slots:            s.Slots,
			NeedHandoff:      s.NeedHandoff,
			AwaitingConfirm:  awaitingConfirm,
			QueryOrderResult: support.ToolResultRecordFromSlots(s.Slots, "query_order"),
		})
		if err != nil {
			return reState{}, err
		}
		s.NeedHandoff = eligibilityResult.NeedHandoff
		s.HandoffReason = eligibilityResult.HandoffReason
		if eligibilityResult.FinalAnswer != "" {
			s.FinalAnswer = eligibilityResult.FinalAnswer
		}
		s.ReadOnly = eligibilityResult.ReadOnly
		s.AwaitingConfirm = eligibilityResult.AwaitingConfirm
		s.ConfirmStatus = eligibilityResult.ConfirmStatus
		return s, nil
	}
}

func reConfirmIfNeeded(confirmNode *aftersalenode.ConfirmSummaryNode) func(context.Context, reState) (reState, error) {
	return func(ctx context.Context, s reState) (reState, error) {
		if !s.AwaitingConfirm {
			return s, nil
		}
		confirmResult, confirmErr := confirmNode.Invoke(ctx, aftersalenode.ConfirmSummaryInput{
			Reply:  s.FinalAnswer,
			Intent: s.Intent,
		})
		if confirmErr != nil {
			return reState{}, confirmErr
		}
		if confirmResult != nil && confirmResult.Reply != "" {
			s.FinalAnswer = confirmResult.Reply
		}
		return s, nil
	}
}

func branchAfterREConfirmPhase(_ context.Context, s reState) (string, error) {
	if strings.EqualFold(s.ConfirmStatus, "confirmed") && !s.NeedHandoff {
		return "ReturnExchangeSubmitToolsNode", nil
	}
	return "ReturnExchangeAssembleOutputNode", nil
}

func reSubmitToolsAndHydrate(registry *agenttool.Registry, toolExec *sharednode.ToolExecNode) func(context.Context, reState) (reState, error) {
	return func(ctx context.Context, s reState) (reState, error) {
		plans, buildErr := buildAfterSaleSubmitPlan(ctx, registry, s.Slots)
		if buildErr != nil {
			return reState{}, buildErr
		}
		if len(plans) == 0 {
			s.NeedHandoff = true
			s.HandoffReason = "after_sale_service_unavailable"
			s.FinalAnswer = "\u552e\u540e\u7533\u8bf7\u670d\u52a1\u6682\u65f6\u4e0d\u53ef\u7528\uff0c\u5df2\u4e3a\u4f60\u8f6c\u4eba\u5de5\u5904\u7406\u3002"
			s.AwaitingConfirm = false
			return s, nil
		}
		if toolExec != nil {
			callMessage, callErr := toolexec.CreateToolCallMessage(plans)
			if callErr != nil {
				return reState{}, callErr
			}
			messages, execErr := toolExec.Invoke(ctx, sharednode.ToolExecutionInput{
				Plans:       plans,
				CallMessage: callMessage,
				Mode:        agenttool.ToolExecutionSerial,
			})
			if execErr != nil {
				return reState{}, execErr
			}
			s.ToolMessages = append(s.ToolMessages, messages...)
			if s.Recorder != nil {
				support.HydrateToolResultsIntoSlots(s.Slots, s.Recorder.Snapshot())
			}
		}
		return s, nil
	}
}

func reRunSubmit(submitNode *aftersalenode.SubmitAfterSaleNode) func(context.Context, reState) (reState, error) {
	return func(ctx context.Context, s reState) (reState, error) {
		if s.NeedHandoff && s.HandoffReason == "after_sale_service_unavailable" {
			return s, nil
		}
		submitResult, submitErr := submitNode.Invoke(ctx, aftersalenode.SubmitAfterSaleInput{
			ConfirmStatus: s.ConfirmStatus,
			RequestType:   support.FirstNonEmpty(slotStringRE(s.Slots, "request_type"), "return"),
			SubmitResult:  support.ToolResultMapFromSlots(s.Slots, "create_after_sale_request"),
		})
		if submitErr != nil {
			return reState{}, submitErr
		}
		s.NeedHandoff = submitResult.NeedHandoff
		s.HandoffReason = submitResult.HandoffReason
		if submitResult.FinalAnswer != "" {
			s.FinalAnswer = submitResult.FinalAnswer
		}
		s.ReadOnly = submitResult.ReadOnly
		s.AwaitingConfirm = submitResult.AwaitingConfirm
		return s, nil
	}
}

func reAssembleOutput(_ context.Context, s reState) (Output, error) {
	return Output{
		FinalAnswer:     s.FinalAnswer,
		NeedHandoff:     s.NeedHandoff,
		HandoffReason:   s.HandoffReason,
		ReadOnly:        s.ReadOnly,
		AwaitingConfirm: s.AwaitingConfirm,
		ToolMessages:    append([]*schema.Message(nil), s.ToolMessages...),
	}, nil
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
		"reason":       slotStringRE(slots, "reason"),
		"request_type": support.FirstNonEmpty(slotStringRE(slots, "request_type"), "return"),
	}
	if slotStringRE(slots, "item_id", "sku_id", "product_id") != "" {
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
	_ = ctx
	return registry.Has(name)
}

func parseSubmitOrderID(slots map[string]any) (int64, error) {
	return strconv.ParseInt(slotStringRE(slots, "order_id"), 10, 64)
}

func parseSubmitItemID(slots map[string]any) (int64, error) {
	return strconv.ParseInt(slotStringRE(slots, "item_id", "sku_id", "product_id"), 10, 64)
}

func cloneSlotsRE(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func slotStringRE(slots map[string]any, keys ...string) string {
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
