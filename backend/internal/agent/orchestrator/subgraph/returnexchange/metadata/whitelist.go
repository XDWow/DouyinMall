package metadata

func AllowedToolNames() []string {
	return []string{"get_order", "list_user_orders", "query_order", "create_after_sale_request"}
}

func AllowedSkillNames() []string {
	return []string{"refund_return"}
}
