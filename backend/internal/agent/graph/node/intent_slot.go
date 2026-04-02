package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type IntentClassifyNode struct{ suite *Suite }
type SlotExtractNode struct{ suite *Suite }
type SlotCheckNode struct{ suite *Suite }
type AskUserNode struct{ suite *Suite }

func (s *Suite) IntentClassify() *IntentClassifyNode { return &IntentClassifyNode{suite: s} }
func (s *Suite) SlotExtract() *SlotExtractNode       { return &SlotExtractNode{suite: s} }
func (s *Suite) SlotCheck() *SlotCheckNode           { return &SlotCheckNode{suite: s} }
func (s *Suite) AskUser() *AskUserNode               { return &AskUserNode{suite: s} }

func (n *IntentClassifyNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	intent := support.HeuristicIntent(flow.State.RawQuery)
	intent.NeedRewrite = support.RequiresRewrite(flow.State.RawQuery, flow.Session)
	if n.suite.deps.Model != nil && n.suite.deps.Prompts != nil && n.suite.deps.Prompts.Intent != nil {
		messages, err := n.suite.deps.Prompts.Intent.Format(ctx, map[string]any{"system_text": n.suite.deps.Prompts.SystemText, "history_text": support.HistoryText(flow.Session, 6), "message": flow.State.RawQuery})
		if err == nil {
			msg, llmErr := n.suite.deps.Model.Generate(ctx, messages, model.WithTemperature(0.1), model.WithMaxTokens(256))
			if llmErr == nil && msg != nil {
				if parsed, ok := support.ParseIntentDecision(msg.Content); ok {
					intent = parsed
				}
			}
		}
	}
	flow.Intent = intent
	flow.State.Intent = intent.Intent
	flow.State.IntentConfidence = intent.Confidence
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *IntentClassifyNode) BuildPromptInput(ctx context.Context, flow *graphstate.FlowContext) (map[string]any, error) {
	graphstate.BindConversationFlow(ctx, flow)
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	return map[string]any{"system_text": n.suite.deps.Prompts.SystemText, "history_text": support.HistoryText(flow.Session, 6), "message": flow.State.RawQuery}, nil
}

func (n *IntentClassifyNode) Apply(ctx context.Context, msg *schema.Message) (*graphstate.FlowContext, error) {
	flow := graphstate.ConversationFlowFromContext(ctx)
	if flow == nil {
		return nil, fmt.Errorf("conversation flow is missing")
	}
	intent := support.HeuristicIntent(flow.State.RawQuery)
	if msg != nil {
		if parsed, ok := support.ParseIntentDecision(msg.Content); ok {
			intent = parsed
		}
	}
	intent.NeedRewrite = intent.NeedRewrite || support.RequiresRewrite(flow.State.RawQuery, flow.Session)
	flow.Intent = intent
	flow.State.Intent = intent.Intent
	flow.State.IntentConfidence = intent.Confidence
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *SlotExtractNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	state := graphstate.EnsureSessionState(flow)
	support.MergeSlots(state.Slots, support.ExtractMetadataSlots(flow.Request.Metadata))
	support.MergeSlots(state.Slots, support.NormalizeEntitySlots(flow.Intent.Entities))
	support.MergeSlots(state.Slots, support.ExtractSlotsFromMessage(flow.State.RawQuery, flow.Intent.Intent))
	if flow.State.ResumeFromCP {
		state.AwaitingUser = false
	}
	if state.AwaitingConfirm {
		state.AwaitingUser = false
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *SlotCheckNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	state := graphstate.EnsureSessionState(flow)
	state.MissingSlots = support.RequiredMissingSlots(flow)
	state.AwaitingUser = len(state.MissingSlots) > 0
	if state.AwaitingUser {
		state.FinalAnswer = support.AskMessageForMissingSlot(flow, state.MissingSlots[0])
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *AskUserNode) Invoke(ctx context.Context, flow *graphstate.FlowContext) (*graphstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	reply := strings.TrimSpace(flow.State.FinalAnswer)
	if reply == "" {
		reply = "Please provide the missing information so I can continue."
	}
	resp := flow.EnsureResponse()
	resp.Reply = reply
	resp.Intent = flow.State.Intent
	resp.Status = dto.ReplyStatusFallback
	resp.Confidence = support.MaxFloat(flow.State.IntentConfidence, 0.8)
	if n.suite.deps.Hooks.PersistConversationTurn != nil {
		if err := n.suite.deps.Hooks.PersistConversationTurn(ctx, flow, reply, resp.Intent, resp.Confidence); err != nil {
			n.suite.deps.Logger.Warn("persist interrupted turn failed", logger.Error(err))
		}
	}
	graphstate.BindConversationFlow(ctx, flow)
	return flow, compose.Interrupt(ctx, map[string]any{"missing_slots": flow.State.MissingSlots, "question": reply})
}
