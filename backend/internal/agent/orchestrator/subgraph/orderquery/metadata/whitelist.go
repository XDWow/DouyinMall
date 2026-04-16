package metadata

func AllowedToolNames() []string {
	return []string{"get_order", "list_user_orders", "query_order"}
}

// ModelAgentToolNames 绑定到子图内「模型 + 工具」节点的业务工具名。
// 订单读由 OrderReadNode 按槽位生成 ToolCallPlan；数字 order_id 来自主图槽位/CurrentRefs 及意图里 order_ref 等指代经 ResolveOrderRefFromTrustedRefs 对齐，避免子图模型在 tool call 里凭记忆写单号；此处返回空，仅保留 fetch_skill（由 SubgraphAgent 在存在技能白名单时自动追加）。
func ModelAgentToolNames() []string {
	return nil
}

func AllowedSkillNames() []string {
	return []string{"order_lookup"}
}
