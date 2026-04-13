package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	inventorynode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/inventory"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	inventorymeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/inventory/metadata"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

// invWire1 槽位校验后：含 Slots 与缺失列表（非空则走追问）。
type invWire1 struct {
	Slots        map[string]any
	MissingSlots []string
}

// invPostAgent 子图模型阶段输出。
type invPostAgent struct {
	invWire1
	AgentFinal  string
	AgentTools  []*schema.Message
	AgentFailed bool
}

func cloneSlotsInv(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func inventoryCheckSlots() func(context.Context, GraphInput) (invWire1, error) {
	return func(_ context.Context, in GraphInput) (invWire1, error) {
		slots := in.Slots
		if slots == nil {
			slots = map[string]any{}
		}
		missing := globalnode.RequiredMissingSlots(domain.IntentInventoryQuery, slots, in.Entities, false)
		return invWire1{Slots: slots, MissingSlots: missing}, nil
	}
}

func branchAfterInventorySlotCheck(_ context.Context, in invWire1) (string, error) {
	if len(in.MissingSlots) > 0 {
		return "InventoryMissingSlotsNode", nil
	}
	return "InventoryModelAgentNode", nil
}

func buildInventoryMissingOutput(_ context.Context, in invWire1) (Output, error) {
	m := in.MissingSlots[0]
	return Output{
		FinalAnswer:  globalnode.AskMessageForMissingSlot(domain.IntentInventoryQuery, m),
		ReadOnly:     true,
		AwaitingUser: true,
		MissingSlots: append([]string(nil), in.MissingSlots...),
	}, nil
}

func runInventoryModelAgent(agent *sharednode.SubgraphAgent, skills *agentskill.Registry) func(context.Context, invWire1) (invPostAgent, error) {
	return func(ctx context.Context, in invWire1) (invPostAgent, error) {
		out := invPostAgent{invWire1: in}
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
			return invPostAgent{}, err
		}
		slotCtx, _ := json.Marshal(in.Slots)
		final, tmsgs, runErr := agent.Run(ctx, sharednode.SubgraphAgentInput{
			ToolNames:    inventorymeta.AllowedToolNames(),
			SkillNames:   skillNames,
			SlotsContext: string(slotCtx),
			UserQuery:    rawQuery,
			History:      history,
			SystemHint:   prompt.SubgraphSystemInventory,
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

func buildInventoryOutputFromAgent(_ context.Context, in invPostAgent) (Output, error) {
	return Output{
		FinalAnswer:  in.AgentFinal,
		ReadOnly:     true,
		ToolMessages: append([]*schema.Message(nil), in.AgentTools...),
	}, nil
}

func runInventoryRulePlanAndTools(
	node *inventorynode.InventoryReadNode,
	toolExec *sharednode.ToolExecNode,
) func(context.Context, invPostAgent) (Output, error) {
	return func(ctx context.Context, in invPostAgent) (Output, error) {
		result, err := node.Invoke(ctx, inventorynode.InventoryReadInput{Slots: in.Slots})
		if err != nil {
			return Output{}, err
		}
		out := Output{
			FinalAnswer:   result.FinalAnswer,
			NeedHandoff:   result.NeedHandoff,
			HandoffReason: result.HandoffReason,
			ReadOnly:      result.ReadOnly,
		}
		if len(result.Plans) == 0 || toolExec == nil {
			return out, nil
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
		out.ToolMessages = append([]*schema.Message(nil), messages...)
		return out, nil
	}
}

func branchAfterInventoryAgent(_ context.Context, in invPostAgent) (string, error) {
	if strings.TrimSpace(in.AgentFinal) != "" {
		return "InventoryAgentAnswerNode", nil
	}
	return "InventoryRulePlanNode", nil
}
