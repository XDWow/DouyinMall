package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	orchestratorobserve "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/observe"
	"github.com/XDWow/DouyinMall/backend/internal/agent/prompt"
)

const defaultSubgraphAgentRounds = 8

// SubgraphAgent 子图内「模型 + 白名单工具多轮」；业务工具与 fetch_skill（拉技能全文）一并绑定，由模型在允许范围内选用。
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

func (a *SubgraphAgent) enabled() bool {
	return a != nil && a.Model != nil
}

// Enabled 是否可走子图内模型 + 工具多轮流程。
func (a *SubgraphAgent) Enabled() bool {
	return a.enabled()
}

// SubgraphAgentInput 单次子图对话入口。
// ToolNames：绑定到模型的业务工具白名单（由 Registry.ToolInfos 解析 schema）。
// SkillNames：技能白名单；摘要注入系统提示，并传入 WithSkillWhitelist；若 Registry 含全文拉取能力则自动并入工具列表。
// SkillText 可选：若非空则额外注入已预加载正文（兼容少数无 Registry 场景）。
type SubgraphAgentInput struct {
	ToolNames     []string
	SkillNames    []string
	SkillText     string
	DocumentsText string
	SlotsContext  string
	UserQuery     string
	History       []*schema.Message
	SystemHint    string
}

func subgraphGenerate(ctx context.Context, m model.ToolCallingChatModel, messages []*schema.Message, opts []model.Option) (*schema.Message, error) {
	var writer domain.StreamWriter
	var traceID string
	_ = domain.ProcessState(ctx, func(s *domain.State) error {
		if s != nil {
			writer = s.StreamWriter
			traceID = s.TraceID
		}
		return nil
	})
	if writer == nil {
		return m.Generate(ctx, messages, opts...)
	}

	reader, err := m.Stream(ctx, messages, opts...)
	if err != nil {
		return m.Generate(ctx, messages, opts...)
	}
	defer reader.Close()

	var chunks []*schema.Message
	emittedText := false
	for {
		chunk, re := reader.Recv()
		if errors.Is(re, io.EOF) {
			break
		}
		if re != nil {
			return nil, re
		}
		if chunk == nil {
			continue
		}
		if txt := strings.TrimSpace(chunk.Content); txt != "" {
			emittedText = true
			orchestratorobserve.SendEvent(ctx, writer, "token", map[string]any{
				"trace_id": traceID,
				"text":     chunk.Content,
			})
		}
		chunks = append(chunks, chunk)
	}
	if len(chunks) == 0 {
		return m.Generate(ctx, messages, opts...)
	}
	out, cErr := schema.ConcatMessages(chunks)
	if cErr != nil {
		return nil, cErr
	}
	if emittedText {
		_ = domain.ProcessState(ctx, func(s *domain.State) error {
			if s != nil {
				s.Answer.Streamed = true
			}
			return nil
		})
	}
	return out, nil
}

func (a *SubgraphAgent) effectiveToolNames(in SubgraphAgentInput) []string {
	names := append([]string(nil), in.ToolNames...)
	if len(in.SkillNames) == 0 || a.Skills == nil || a.Registry == nil || !a.Registry.Has("fetch_skill") {
		return names
	}
	names = append(names, "fetch_skill")
	return names
}

// Run 返回模型最终自然语言与本轮产生的 tool 消息（供主图 Hydrate）。
func (a *SubgraphAgent) Run(ctx context.Context, in SubgraphAgentInput) (final string, toolMsgs []*schema.Message, err error) {
	if !a.enabled() {
		return "", nil, nil
	}
	toolNames := a.effectiveToolNames(in)
	if len(toolNames) > 0 && a.Registry == nil {
		return "", nil, fmt.Errorf("tool registry required when tools are non-empty")
	}
	ctx = agenttool.WithSkillWhitelist(ctx, in.SkillNames)

	rounds := a.MaxRounds
	if rounds <= 0 {
		rounds = defaultSubgraphAgentRounds
	}

	var sys strings.Builder
	if strings.TrimSpace(in.SystemHint) != "" {
		sys.WriteString(strings.TrimSpace(in.SystemHint))
		sys.WriteString("\n\n")
	}
	if strings.TrimSpace(in.SlotsContext) != "" {
		sys.WriteString("已知槽位（供工具参数参考）：\n")
		sys.WriteString(strings.TrimSpace(in.SlotsContext))
		sys.WriteString("\n\n")
	}
	if strings.TrimSpace(in.DocumentsText) != "" {
		sys.WriteString("参考资料：\n")
		sys.WriteString(strings.TrimSpace(in.DocumentsText))
		sys.WriteString("\n\n")
	}
	if len(in.SkillNames) > 0 && a.Skills != nil {
		sums := a.Skills.SummariesByNames(in.SkillNames)
		if cat := agentskill.RenderSkillSummaryText(sums); cat != "" && cat != "none" {
			sys.WriteString("以下为当前场景可用的技能条目（名称与摘要）。需要完整条文时，仅可从下列名称中选取并按规定方式获取全文；参数须与名称一致：\n")
			sys.WriteString(cat)
			sys.WriteString("\n\n")
		}
	}
	if strings.TrimSpace(in.SkillText) != "" && strings.TrimSpace(in.SkillText) != "none" {
		sys.WriteString("业务技能说明（已预加载正文）：\n")
		sys.WriteString(strings.TrimSpace(in.SkillText))
		sys.WriteString("\n\n")
	}
	system := strings.TrimSpace(sys.String())
	if system == "" {
		system = prompt.SubgraphSystemDefault
	}

	messages := []*schema.Message{schema.SystemMessage(system)}
	messages = append(messages, append([]*schema.Message(nil), in.History...)...)
	if q := strings.TrimSpace(in.UserQuery); q != "" {
		messages = append(messages, schema.UserMessage(q))
	}

	var toolInfos []*schema.ToolInfo
	if len(toolNames) > 0 {
		toolInfos = a.Registry.ToolInfos(toolNames)
	}
	activeModel := a.Model
	if len(toolInfos) > 0 {
		bound, werr := a.Model.WithTools(toolInfos)
		if werr != nil {
			return "", nil, werr
		}
		activeModel = bound
	}

	var toolsNode *compose.ToolsNode
	if len(toolInfos) > 0 {
		var terr error
		toolsNode, terr = a.Registry.ToolsNode(agenttool.ToolExecutionSerial)
		if terr != nil {
			return "", nil, terr
		}
	}

	opts := []model.Option{model.WithTemperature(0.2)}
	if a.MaxTokens > 0 {
		opts = append(opts, model.WithMaxTokens(a.MaxTokens))
	}
	if len(toolInfos) == 0 {
		opts = append(opts, model.WithToolChoice(schema.ToolChoiceForbidden))
	}

	var allTool []*schema.Message
	for range rounds {
		out, genErr := subgraphGenerate(ctx, activeModel, messages, opts)
		if genErr != nil {
			return "", allTool, genErr
		}
		if out == nil {
			return "", allTool, fmt.Errorf("model returned nil message")
		}
		if out.Role == "" {
			out.Role = schema.Assistant
		}
		messages = append(messages, out)
		if len(out.ToolCalls) == 0 {
			return strings.TrimSpace(out.Content), allTool, nil
		}
		if toolsNode == nil {
			return "", allTool, fmt.Errorf("model requested tools but tools node is nil")
		}
		tOut, invErr := toolsNode.Invoke(ctx, out)
		if invErr != nil {
			return "", allTool, invErr
		}
		allTool = append(allTool, tOut...)
		messages = append(messages, tOut...)
	}
	return "", allTool, fmt.Errorf("subgraph agent exceeded max rounds (%d)", rounds)
}
