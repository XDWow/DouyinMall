package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

// ResponseRenderNode 负责整理最终回复，并在需要时调用模型润色答案。
// 命中的 skill 内容不是让模型临场挑选，而是由前面的 route 编排先收敛后再注入。
// 这样既能减少 token，也能减少无关技能对回答的干扰。
type ResponseRenderNode struct {
	Model          model.ToolCallingChatModel
	Prompts        *orchestratorprompt.Set
	Skills         *agentskill.Registry
	GenerateAnswer AnswerGenerator
	Metrics        *orchestratorobserve.Metrics
}

func NewResponseRenderNode(
	chatModel model.ToolCallingChatModel,
	prompts *orchestratorprompt.Set,
	skills *agentskill.Registry,
	generateAnswer AnswerGenerator,
	metrics *orchestratorobserve.Metrics,
) *ResponseRenderNode {
	return &ResponseRenderNode{
		Model:          chatModel,
		Prompts:        prompts,
		Skills:         skills,
		GenerateAnswer: generateAnswer,
		Metrics:        metrics,
	}
}

func (n *ResponseRenderNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is required")
	}

	resp := state.EnsureResponse()
	resp.SessionID = state.Session.SessionID
	if state.Session.CacheHitLevel != "" && strings.TrimSpace(resp.Reply) != "" {
		resp.Status = domain.ReplyStatusAnswered
		resp.Intent = state.Session.Intent
		return state, nil
	}

	reply := strings.TrimSpace(state.Session.FinalAnswer)
	source := "state"
	if reply == "" && support.ShouldUseLLMAnswer(state) && n.Model != nil && n.Prompts != nil && n.Prompts.Answer != nil {
		messages, err := n.Prompts.Answer.Format(ctx, map[string]any{
			"system_text":    n.Prompts.SystemText,
			"history":        graphstate.RecentMessages(state),
			"message":        state.Session.RawQuery,
			"query":          support.FirstNonEmpty(state.Rewrite.Query, state.Session.RawQuery),
			"documents_text": support.DocumentsText(state.Retrieval.Documents),
			"tool_text":      support.ToolText(state.ToolExecutions()),
			"skill_text":     n.selectedSkillText(state.Skill.Names),
		})
		if err == nil && n.GenerateAnswer != nil {
			if generated, genErr := n.GenerateAnswer(ctx, state, messages); genErr == nil && strings.TrimSpace(generated) != "" {
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
	state.Answer = graphstate.AnswerResult{
		Reply:      reply,
		Confidence: support.EstimateConfidence(state),
		Source:     source,
	}
	resp.Reply = reply
	resp.Intent = state.Session.Intent
	resp.Confidence = state.Answer.Confidence
	resp.References = support.DocumentsToReferences(state.Retrieval.Documents)
	resp.ToolExecutions = state.ToolExecutions()
	resp.NeedHandoff = state.Session.NeedHandoff
	resp.HandoffReason = state.Session.HandoffReason
	resp.Trace.RewrittenQuery = state.Rewrite.Query

	if resp.NeedHandoff {
		resp.Status = domain.ReplyStatusHandoff
		if n.Metrics != nil {
			n.Metrics.ObserveHandoff(resp.HandoffReason)
		}
	} else {
		resp.Status = domain.ReplyStatusAnswered
	}

	return state, nil
}

func (n *ResponseRenderNode) selectedSkillText(names []string) string {
	if n.Skills == nil {
		return "none"
	}
	return agentskill.RenderSkillText(n.Skills.Load(names))
}
