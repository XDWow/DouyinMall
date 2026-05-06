package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	adkskill "github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
)

const defaultSubgraphAgentRounds = 8

// SubgraphAgent is the shared ADK-based agent wrapper used by read-only or ambiguous subgraphs.
// The outer orchestration still stays in compose workflow.
// Only the inner model/tool/skill loop is delegated to ADK.
type SubgraphAgent struct {
	Model     model.ToolCallingChatModel
	Registry  *agenttool.Registry
	Skills    *agentskill.Registry
	MaxRounds int
	MaxTokens int
}

func NewSubgraphAgent(m model.ToolCallingChatModel, reg *agenttool.Registry, skills *agentskill.Registry, maxTokens int) *SubgraphAgent {
	return &SubgraphAgent{
		Model:     m,
		Registry:  reg,
		Skills:    skills,
		MaxRounds: defaultSubgraphAgentRounds,
		MaxTokens: maxTokens,
	}
}

func (a *SubgraphAgent) Enabled() bool {
	return a != nil && a.Model != nil
}

// SubgraphAgentInput is the only context an agent subgraph may pass into the ADK agent.
// ToolNames and SkillNames are explicit business whitelists.
type SubgraphAgentInput struct {
	ToolNames     []string
	SkillNames    []string
	DocumentsText string
	SlotsContext  string
	UserQuery     string
	History       []*schema.Message
	SystemHint    string
}

func (a *SubgraphAgent) Run(ctx context.Context, in SubgraphAgentInput) (string, []*schema.Message, error) {
	if !a.Enabled() {
		return "", nil, nil
	}

	tools, err := a.lookupTools(in.ToolNames)
	if err != nil {
		return "", nil, err
	}
	middlewares, err := a.buildSkillMiddlewares(ctx, in.SkillNames)
	if err != nil {
		return "", nil, err
	}

	agent, err := a.buildChatAgent(ctx, buildInstruction(in), tools, middlewares)
	if err != nil {
		return "", nil, err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	iter := runner.Run(ctx, buildInputMessages(in), a.modelRunOptions()...)
	return collectRunnerOutput(ctx, iter)
}

func (a *SubgraphAgent) lookupTools(names []string) ([]einotool.BaseTool, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if a.Registry == nil {
		return nil, fmt.Errorf("tool registry required when tools are non-empty")
	}
	return a.Registry.Tools(names), nil
}

func (a *SubgraphAgent) buildSkillMiddlewares(ctx context.Context, skillNames []string) ([]adk.ChatModelAgentMiddleware, error) {
	if len(skillNames) == 0 {
		return nil, nil
	}
	if a.Skills == nil {
		return nil, fmt.Errorf("skill registry required when skills are non-empty")
	}

	backend := a.Skills.ADKBackend(skillNames)
	if backend == nil {
		return nil, fmt.Errorf("skill backend is empty for %v", skillNames)
	}

	handler, err := adkskill.NewMiddleware(ctx, &adkskill.Config{
		Backend: backend,
	})
	if err != nil {
		return nil, fmt.Errorf("build skill middleware: %w", err)
	}
	return []adk.ChatModelAgentMiddleware{handler}, nil
}

func (a *SubgraphAgent) buildChatAgent(
	ctx context.Context,
	instruction string,
	tools []einotool.BaseTool,
	middlewares []adk.ChatModelAgentMiddleware,
) (*adk.ChatModelAgent, error) {
	rounds := a.MaxRounds
	if rounds <= 0 {
		rounds = defaultSubgraphAgentRounds
	}

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "SubgraphAgentNode",
		Description:   "Agent node inside customer-service subgraph.",
		Instruction:   instruction,
		Model:         a.Model,
		MaxIterations: rounds,
		GenModelInput: func(_ context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {
			msgs := make([]adk.Message, 0, len(input.Messages)+1)
			if text := strings.TrimSpace(instruction); text != "" {
				msgs = append(msgs, schema.SystemMessage(text))
			}
			msgs = append(msgs, input.Messages...)
			return msgs, nil
		},
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               tools,
				ExecuteSequentially: true,
			},
		},
		Handlers: middlewares,
	})
}

func (a *SubgraphAgent) modelRunOptions() []adk.AgentRunOption {
	opts := []model.Option{model.WithTemperature(0.2)}
	if a.MaxTokens > 0 {
		opts = append(opts, model.WithMaxTokens(a.MaxTokens))
	}
	return []adk.AgentRunOption{adk.WithChatModelOptions(opts)}
}

func buildInstruction(in SubgraphAgentInput) string {
	var sys strings.Builder
	if strings.TrimSpace(in.SystemHint) != "" {
		sys.WriteString(strings.TrimSpace(in.SystemHint))
		sys.WriteString("\n\n")
	}
	if strings.TrimSpace(in.SlotsContext) != "" {
		sys.WriteString("<known_slots>\n")
		sys.WriteString(strings.TrimSpace(in.SlotsContext))
		sys.WriteString("\n</known_slots>\n\n")
	}
	if strings.TrimSpace(in.DocumentsText) != "" {
		sys.WriteString("<retrieved_documents>\n")
		sys.WriteString(strings.TrimSpace(in.DocumentsText))
		sys.WriteString("\n</retrieved_documents>\n\n")
	}

	system := strings.TrimSpace(sys.String())
	if system == "" {
		system = strings.TrimSpace(defaultSubgraphSystemPrompt)
	}
	return system
}

func buildInputMessages(in SubgraphAgentInput) []adk.Message {
	messages := make([]adk.Message, 0, len(in.History)+1)
	messages = append(messages, append([]*schema.Message(nil), in.History...)...)
	if q := strings.TrimSpace(in.UserQuery); q != "" {
		messages = append(messages, schema.UserMessage("<user_query>\n"+q+"\n</user_query>"))
	}
	return messages
}

func collectRunnerOutput(ctx context.Context, iter *adk.AsyncIterator[*adk.AgentEvent]) (string, []*schema.Message, error) {
	var final string
	var toolMsgs []*schema.Message

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", toolMsgs, event.Err
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			return "", toolMsgs, subgraphcommon.InterruptForDecision(ctx, extractInterruptDecision(event.Action.Interrupted))
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return "", toolMsgs, err
		}
		if msg == nil {
			continue
		}
		if event.Output.MessageOutput.Role == schema.Tool {
			toolMsgs = append(toolMsgs, msg)
			continue
		}
		if text := strings.TrimSpace(msg.Content); text != "" {
			final = text
		}
	}

	return final, toolMsgs, nil
}

var defaultInterruptDecision = subgraphcommon.AgentDecision{
	Type:     "clarification",
	Question: "Please provide more detail.",
}

func extractInterruptDecision(info *adk.InterruptInfo) subgraphcommon.AgentDecision {
	if info == nil {
		return defaultInterruptDecision
	}
	for _, ic := range info.InterruptContexts {
		if ic != nil && ic.IsRootCause {
			if decision := parseInterruptDetail(ic.Info); decision.Type != "" {
				return decision
			}
		}
	}
	for _, ic := range info.InterruptContexts {
		if ic != nil {
			if decision := parseInterruptDetail(ic.Info); decision.Type != "" {
				return decision
			}
		}
	}
	return defaultInterruptDecision
}

func parseInterruptDetail(raw any) subgraphcommon.AgentDecision {
	detail, ok := raw.(map[string]any)
	if !ok {
		return subgraphcommon.AgentDecision{}
	}

	decision := subgraphcommon.AgentDecision{
		Type:     strings.TrimSpace(fmt.Sprint(detail["type"])),
		Reply:    strings.TrimSpace(fmt.Sprint(detail["reply"])),
		Question: strings.TrimSpace(fmt.Sprint(detail["question"])),
	}
	if fields, ok := detail["missing_fields"].([]string); ok {
		decision.MissingFields = append([]string(nil), fields...)
		return decision
	}
	if values, ok := detail["missing_fields"].([]any); ok {
		decision.MissingFields = make([]string, 0, len(values))
		for _, value := range values {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				decision.MissingFields = append(decision.MissingFields, text)
			}
		}
	}
	return decision
}
