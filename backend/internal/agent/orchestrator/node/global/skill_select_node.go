package global

import (
	"context"

	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	subgraphmeta "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/metadata"
)

// SkillSelectNode 鍦?route 宸茬粡纭畾鍚庯紝鏀舵暃褰撳墠璇锋眰鐪熸闇€瑕佺殑 skill 闆嗗悎銆?
// 杩欐槸涓€娆℃槑纭殑缂栨帓鍐崇瓥锛氳兘鐢变笟鍔¤鍒欑‘瀹氱殑鍐呭锛屽氨涓嶈鍐嶄氦缁欐ā鍨嬪幓鐚溿€?
// 宸ュ叿鏆撮湶鑼冨洿宸茬粡鐢?route 瀵瑰簲鐨勫瓙鍥惧ぉ鐒堕檺鍒讹紝杩欓噷鍙礋璐ｆ妧鑳芥鏂囩殑娉ㄥ叆鑼冨洿銆?
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
	names := subgraphmeta.Resolve(input.Route).SkillNames
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
