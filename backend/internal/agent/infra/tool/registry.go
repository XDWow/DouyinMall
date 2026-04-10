package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

// Registry 负责工具的注册期能力：
// 1. 保存已发现的工具集合和元数据。
// 2. 暴露串行/并行 ToolNode 给编排层复用。
// 3. 提供按名字查询 tool summary 的能力，方便子图按需注入。
type Registry struct {
	tools          []einotool.BaseTool
	invokables     map[string]registeredInvokableTool
	summaries      map[string]ToolSummary
	toolInfos      map[string]*schema.ToolInfo
	sequentialNode *compose.ToolsNode
	parallelNode   *compose.ToolsNode
}

type registeredInvokableTool struct {
	invokable einotool.InvokableTool
	policy    ToolPolicy
}

type registeredTool struct {
	baseTool  einotool.BaseTool
	invokable einotool.InvokableTool
	info      *schema.ToolInfo
	policy    ToolPolicy
}

func newRegistry(ctx context.Context, registered []registeredTool) (*Registry, error) {
	tools := make([]einotool.BaseTool, 0, len(registered))
	invokables := make(map[string]registeredInvokableTool, len(registered))
	summaries := make(map[string]ToolSummary, len(registered))
	toolInfos := make(map[string]*schema.ToolInfo, len(registered))

	for _, item := range registered {
		if item.baseTool == nil || item.info == nil || strings.TrimSpace(item.info.Name) == "" {
			continue
		}
		tools = append(tools, item.baseTool)
		invokables[item.info.Name] = registeredInvokableTool{
			invokable: item.invokable,
			policy:    item.policy,
		}
		summaries[item.info.Name] = buildToolSummary(item.info, item.policy)
		toolInfos[item.info.Name] = item.info
	}

	sequentialNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools:               tools,
		ExecuteSequentially: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create sequential tools node: %w", err)
	}
	parallelNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools:               tools,
		ExecuteSequentially: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create parallel tools node: %w", err)
	}

	return &Registry{
		tools:          tools,
		invokables:     invokables,
		summaries:      summaries,
		toolInfos:      toolInfos,
		sequentialNode: sequentialNode,
		parallelNode:   parallelNode,
	}, nil
}

// ToolInfos 按白名单名返回已注册的 ToolInfo，供子图 WithTools 绑定模型；未知名跳过。
func (r *Registry) ToolInfos(names []string) []*schema.ToolInfo {
	if r == nil || len(r.toolInfos) == 0 || len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]*schema.ToolInfo, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if info := r.toolInfos[name]; info != nil {
			out = append(out, info)
		}
	}
	return out
}

func (r *Registry) Tools() []einotool.BaseTool {
	return r.tools
}

func (r *Registry) ToolsNode(mode ToolExecutionMode) (*compose.ToolsNode, error) {
	if r == nil {
		return nil, fmt.Errorf("tool registry is not initialized")
	}
	switch mode {
	case ToolExecutionParallelReadOnly:
		if r.parallelNode == nil {
			return nil, fmt.Errorf("parallel tools node is not initialized")
		}
		return r.parallelNode, nil
	default:
		if r.sequentialNode == nil {
			return nil, fmt.Errorf("sequential tools node is not initialized")
		}
		return r.sequentialNode, nil
	}
}

func (r *Registry) Policy(name string) (ToolPolicy, bool) {
	if r == nil || r.invokables == nil {
		return ToolPolicy{}, false
	}
	item, ok := r.invokables[name]
	if !ok {
		return ToolPolicy{}, false
	}
	return item.policy, true
}

func (r *Registry) Has(name string) bool {
	if r == nil || r.summaries == nil {
		return false
	}
	_, ok := r.summaries[strings.TrimSpace(name)]
	return ok
}

func (r *Registry) Summary(name string) (ToolSummary, bool) {
	if r == nil || r.summaries == nil {
		return ToolSummary{}, false
	}
	item, ok := r.summaries[strings.TrimSpace(name)]
	return item, ok
}

func (r *Registry) Summaries(names []string) []ToolSummary {
	if r == nil || len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	items := make([]ToolSummary, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		item, ok := r.summaries[name]
		if !ok {
			continue
		}
		seen[name] = struct{}{}
		items = append(items, item)
	}
	return items
}

func (r *Registry) SummariesFromPlans(plans []domain.ToolCallPlan) []ToolSummary {
	if len(plans) == 0 {
		return nil
	}
	names := make([]string, 0, len(plans))
	for _, plan := range plans {
		names = append(names, plan.Name)
	}
	return r.Summaries(names)
}

func (r *Registry) ValidatePlans(plans []domain.ToolCallPlan, mode ToolExecutionMode) error {
	if len(plans) == 0 || mode != ToolExecutionParallelReadOnly {
		return nil
	}
	for _, plan := range plans {
		policy, ok := r.Policy(plan.Name)
		if !ok {
			return fmt.Errorf("tool not registered: %s", plan.Name)
		}
		if !policy.ReadOnly || policy.RequiresOrdering {
			return fmt.Errorf("tool %s cannot run in parallel readonly mode", plan.Name)
		}
	}
	return nil
}

func RenderToolSummaryText(items []ToolSummary) string {
	if len(items) == 0 {
		return "none"
	}
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString("## ")
		builder.WriteString(item.Name)
		builder.WriteString("\n")
		if item.Description != "" {
			builder.WriteString("说明：")
			builder.WriteString(strings.TrimSpace(item.Description))
			builder.WriteString("\n")
		}
		builder.WriteString("读写属性：")
		if item.ReadOnly {
			builder.WriteString("只读")
		} else {
			builder.WriteString("写操作")
		}
		builder.WriteString("\n")
		if strings.TrimSpace(item.InputSchema) != "" {
			builder.WriteString("输入定义：")
			builder.WriteString("\n")
			builder.WriteString(item.InputSchema)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func buildToolSummary(info *schema.ToolInfo, policy ToolPolicy) ToolSummary {
	if info == nil {
		return ToolSummary{}
	}
	return ToolSummary{
		Name:             strings.TrimSpace(info.Name),
		Description:      strings.TrimSpace(info.Desc),
		InputSchema:      renderToolInputSchema(info),
		ReadOnly:         policy.ReadOnly,
		RequiresOrdering: policy.RequiresOrdering,
	}
}

func renderToolInputSchema(info *schema.ToolInfo) string {
	if info == nil || info.ParamsOneOf == nil {
		return ""
	}
	s, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil || s == nil {
		return ""
	}
	payload, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(payload))
}
