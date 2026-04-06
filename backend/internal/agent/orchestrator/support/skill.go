package support

import orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"

// SkillNamesForRoute 返回当前 route 下允许注入给模型的 skill 名称。
// 这里刻意只覆盖“我们能确定”的场景，避免在 fallback 等模糊问题里
// 把大段技能正文无差别注入给模型，浪费 token 还会增加干扰。
func SkillNamesForRoute(route orchestratorstate.WorkflowRoute) []string {
	switch route {
	case orchestratorstate.RouteOrderQuery:
		return []string{"order_lookup", "logistics_exception"}
	case orchestratorstate.RouteReturnPolicy, orchestratorstate.RouteReturnExchangeApply:
		return []string{"refund_return"}
	default:
		return nil
	}
}
