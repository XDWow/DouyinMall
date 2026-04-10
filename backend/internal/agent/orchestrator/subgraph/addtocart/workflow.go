package addtocart

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	cartnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/cart"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	addtocartmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart/metadata"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

type cartWire1 struct {
	Slots        map[string]any
	MissingSlots []string
}

type cartPostAgent struct {
	cartWire1
	AgentFinal  string
	AgentTools  []*schema.Message
	AgentFailed bool
}

func cloneSlotsCart(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func addToCartCheckSlots() func(context.Context, struct{}) (cartWire1, error) {
	return func(ctx context.Context, _ struct{}) (cartWire1, error) {
		var slots map[string]any
		var entities map[string]string
		if err := domain.ProcessState(ctx, func(s *domain.State) error {
			if s == nil {
				return fmt.Errorf("state is nil")
			}
			slots = cloneSlotsCart(s.Session.Slots)
			entities = s.Intent.Entities
			globalnode.ApplyIntentFieldsForTools(slots, entities)
			return nil
		}); err != nil {
			return cartWire1{}, err
		}
		if slots == nil {
			slots = map[string]any{}
		}
		missing := globalnode.RequiredMissingSlots(domain.IntentAddToCart, slots, entities, false)
		return cartWire1{Slots: slots, MissingSlots: missing}, nil
	}
}

func branchAfterCartSlotCheck(_ context.Context, in cartWire1) (string, error) {
	if len(in.MissingSlots) > 0 {
		return "AddToCartMissingSlotsNode", nil
	}
	return "AddToCartModelAgentNode", nil
}

func buildCartMissingOutput(_ context.Context, in cartWire1) (Output, error) {
	m := in.MissingSlots[0]
	return Output{
		FinalAnswer:  globalnode.AskMessageForMissingSlot(domain.IntentAddToCart, m),
		ReadOnly:     true,
		AwaitingUser: true,
		MissingSlots: append([]string(nil), in.MissingSlots...),
	}, nil
}

func runAddToCartModelAgent(agent *sharednode.SubgraphAgent, skills *agentskill.Registry) func(context.Context, cartWire1) (cartPostAgent, error) {
	return func(ctx context.Context, in cartWire1) (cartPostAgent, error) {
		out := cartPostAgent{cartWire1: in}
		if agent == nil || !agent.Enabled() {
			return out, nil
		}
		var rawQuery string
		var history []*schema.Message
		var skillNames []string
		if err := domain.ProcessState(ctx, func(s *domain.State) error {
			if s == nil {
				return fmt.Errorf("state is nil")
			}
			rawQuery = s.Session.RawQuery
			history = append([]*schema.Message(nil), s.Session.Messages...)
			skillNames = subgraphmeta.FilteredSkillNames(s.Session.Route, skills)
			return nil
		}); err != nil {
			return cartPostAgent{}, err
		}
		slotCtx, _ := json.Marshal(in.Slots)
		final, tmsgs, runErr := agent.Run(ctx, sharednode.SubgraphAgentInput{
			ToolNames:    addtocartmeta.AllowedToolNames(),
			SkillNames:   skillNames,
			SlotsContext: string(slotCtx),
			UserQuery:    rawQuery,
			History:      history,
			SystemHint:   prompt.SubgraphSystemAddToCart,
		})
		if runErr != nil {
			out.AgentFailed = true
			return out, nil
		}
		out.AgentFinal = strings.TrimSpace(final)
		out.AgentTools = append([]*schema.Message(nil), tmsgs...)
		return out, nil
	}
}

func applyAddToCartSuccessTemplate(slots map[string]any, out *Output) {
	if strings.TrimSpace(out.FinalAnswer) != "" {
		return
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
}

func hydrateAndTemplateCart(ctx context.Context, slots map[string]any, base Output) (Output, error) {
	_ = domain.ProcessState(ctx, func(s *domain.State) error {
		if s != nil && s.Recorder != nil {
			support.HydrateToolResultsIntoSlots(slots, s.Recorder.Snapshot())
		}
		return nil
	})
	applyAddToCartSuccessTemplate(slots, &base)
	return base, nil
}

func buildCartOutputFromAgent(ctx context.Context, in cartPostAgent) (Output, error) {
	base := Output{
		FinalAnswer:  in.AgentFinal,
		ReadOnly:     false,
		ToolMessages: append([]*schema.Message(nil), in.AgentTools...),
	}
	return hydrateAndTemplateCart(ctx, in.Slots, base)
}

func runCartRulePlanAndTools(
	node *cartnode.AddToCartNode,
	toolExec *sharednode.ToolExecNode,
) func(context.Context, cartPostAgent) (Output, error) {
	return func(ctx context.Context, in cartPostAgent) (Output, error) {
		result, err := node.Invoke(ctx, cartnode.AddToCartInput{Slots: in.Slots})
		if err != nil {
			return Output{}, err
		}
		base := Output{
			FinalAnswer:   result.FinalAnswer,
			NeedHandoff:   result.NeedHandoff,
			HandoffReason: result.HandoffReason,
			ReadOnly:      result.ReadOnly,
		}
		if len(result.Plans) == 0 || toolExec == nil {
			return hydrateAndTemplateCart(ctx, in.Slots, base)
		}
		callMessage, err := toolexec.CreateToolCallMessage(result.Plans)
		if err != nil {
			return Output{}, err
		}
		messages, err := toolExec.Invoke(ctx, sharednode.ToolExecutionInput{
			Plans:       result.Plans,
			CallMessage: callMessage,
			Mode:        agenttool.ToolExecutionSerial,
		})
		if err != nil {
			return Output{}, err
		}
		base.ToolMessages = append([]*schema.Message(nil), messages...)
		return hydrateAndTemplateCart(ctx, in.Slots, base)
	}
}

func branchAfterCartAgent(_ context.Context, in cartPostAgent) (string, error) {
	if strings.TrimSpace(in.AgentFinal) != "" {
		return "AddToCartAgentAnswerNode", nil
	}
	return "AddToCartRulePlanNode", nil
}
