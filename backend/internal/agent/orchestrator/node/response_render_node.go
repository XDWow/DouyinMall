package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type ResponseRenderNodeDeps struct {
	Model          model.ToolCallingChatModel
	Prompts        *orchestratorprompt.Set
	GenerateAnswer AnswerGenerator
	Metrics        *observe.Metrics
}

type ResponseRenderNode struct{ deps ResponseRenderNodeDeps }

func NewResponseRenderNode(deps ResponseRenderNodeDeps) *ResponseRenderNode {
	return &ResponseRenderNode{deps: deps}
}

func (n *ResponseRenderNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}
	resp := state.EnsureResponse()
	resp.SessionID = state.Session.SessionID
	if state.Session.CacheHitLevel == "L0" && strings.TrimSpace(resp.Reply) != "" {
		resp.Status = domain.ReplyStatusAnswered
		resp.Intent = state.Session.Intent
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}
	reply := strings.TrimSpace(state.Session.FinalAnswer)
	source := "state"
	if reply == "" && support.ShouldUseLLMAnswer(state) && n.deps.Model != nil && n.deps.Prompts != nil && n.deps.Prompts.Answer != nil {
		messages, err := n.deps.Prompts.Answer.Format(ctx, map[string]any{
			"system_text":     n.deps.Prompts.SystemText,
			"history":         graphstate.RecentMessages(state),
			"message":         state.Session.RawQuery,
			"query":           support.FirstNonEmpty(state.Session.RewrittenQuery, state.Session.RawQuery),
			"references_text": support.ReferencesText(state.Retrieval.References),
			"tool_text":       support.ToolText(state.ToolExecutions()),
		})
		if err == nil && n.deps.GenerateAnswer != nil {
			if generated, genErr := n.deps.GenerateAnswer(ctx, state, messages); genErr == nil && strings.TrimSpace(generated) != "" {
				reply = generated
				source = "llm"
			}
		}
	}
	if strings.TrimSpace(reply) == "" {
		reply = support.TemplateAnswer(state)
		source = "template"
	}
	reply = support.NormalizeReply(reply)
	state.Answer = graphstate.AnswerResult{Reply: reply, Confidence: support.EstimateConfidence(state), Source: source}
	resp.Reply = reply
	resp.Intent = state.Session.Intent
	resp.Confidence = state.Answer.Confidence
	resp.References = state.Retrieval.References
	resp.ToolExecutions = state.ToolExecutions()
	resp.NeedHandoff = state.Session.NeedHandoff
	resp.HandoffReason = state.Session.HandoffReason
	resp.Trace.RewrittenQuery = state.Session.RewrittenQuery
	if resp.NeedHandoff {
		resp.Status = domain.ReplyStatusHandoff
		if n.deps.Metrics != nil {
			n.deps.Metrics.ObserveHandoff(resp.HandoffReason)
		}
	} else {
		resp.Status = domain.ReplyStatusAnswered
	}
	graphstate.BindConversationState(ctx, state)
	return state, nil
}
