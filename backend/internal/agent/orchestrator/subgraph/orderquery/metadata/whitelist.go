package metadata

func AllowedToolNames() []string {
	return []string{"get_order", "list_user_orders", "query_order"}
}

func AllowedSkillNames() []string {
	return []string{"order_lookup", "logistics_exception"}
}
