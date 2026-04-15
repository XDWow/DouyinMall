package orderquery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	ordernode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/order"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
	orderquerymeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderquery/metadata"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

// midSlots PrepareSlots 节点输出。
type midSlots struct {
	Slots map[string]any
}

// postAgent ModelAgent 节点输出。
type postAgent struct {
	midSlots
	AgentFinal  string
	AgentTools  []*schema.Message
	AgentFailed bool
}

func cloneSlotsOrder(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func prepareSlots() func(context.Context, GraphInput) (midSlots, error) {
	return func(_ context.Context, in GraphInput) (midSlots, error) {
		slots := in.Slots
		if slots == nil {
			slots = map[string]any{}
		}
		return midSlots{Slots: slots}, nil
	}
}

func runOrderModelAgent(agent *sharednode.SubgraphAgent, skills *agentskill.Registry) func(context.Context, midSlots) (postAgent, error) {
	return func(ctx context.Context, in midSlots) (postAgent, error) {
		out := postAgent{midSlots: in}
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
			return postAgent{}, err
		}
		slotCtx, _ := json.Marshal(in.Slots)
		final, tmsgs, runErr := agent.Run(ctx, sharednode.SubgraphAgentInput{
			ToolNames:    orderquerymeta.ModelAgentToolNames(),
			SkillNames:   skillNames,
			SlotsContext: string(slotCtx),
			UserQuery:    rawQuery,
			History:      history,
			SystemHint:   prompt.SubgraphSystemOrderQuery,
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

func runOrderRulePlanAndTools(
	node *ordernode.OrderReadNode,
	toolExec *sharednode.ToolExecNode,
) func(context.Context, postAgent) (Output, error) {
	return func(ctx context.Context, in postAgent) (Output, error) {
		result, err := node.Invoke(ctx, ordernode.OrderReadInput{Slots: in.Slots})
		if err != nil {
			return Output{}, err
		}
		out := Output{
			FinalAnswer:   result.FinalAnswer,
			NeedHandoff:   result.NeedHandoff,
			HandoffReason: result.HandoffReason,
			ReadOnly:      result.ReadOnly,
			ToolMessages:  append([]*schema.Message(nil), in.AgentTools...),
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
		out.ToolMessages = append(append([]*schema.Message(nil), in.AgentTools...), messages...)
		return out, nil
	}
}
