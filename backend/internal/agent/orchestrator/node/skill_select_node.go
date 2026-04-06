package node

import (
	"context"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// SkillSelectNode 在 route 已经确定后，收敛当前请求真正需要的 skill 集合。
// 这是一次明确的编排决策：能由业务规则确定的内容，就不要再交给模型去猜。
// 工具暴露范围已经由 route 对应的子图天然限制，这里只负责技能正文的注入范围。
type SkillSelectNode struct {
	Registry *agentskill.Registry
}

func NewSkillSelectNode(registry *agentskill.Registry) *SkillSelectNode {
	return &SkillSelectNode{Registry: registry}
}

type SkillSelectInput struct {
	Route graphstate.WorkflowRoute
}

type SkillSelectResult struct {
	Names []string
}

func (n *SkillSelectNode) Invoke(_ context.Context, input SkillSelectInput) (*SkillSelectResult, error) {
	names := support.SkillNamesForRoute(input.Route)
	if n.Registry == nil || len(names) == 0 {
		return &SkillSelectResult{Names: names}, nil
	}

	items := n.Registry.Load(names)
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Name)
	}
	return &SkillSelectResult{Names: result}, nil
}
