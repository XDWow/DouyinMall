package mcp

// listUserOrdersArgs 列表工具无业务入参；用户身份来自 mcpruntime。
type listUserOrdersArgs struct{}

// getOrderArgs 单笔查询入参（与工具 schema 字段名一致）。
type getOrderArgs struct {
	OrderID int64 `json:"order_id"`
}

// QueryOrderItemView 返回给 Agent 的订单摘要视图，与 domain / proto 解耦。
type QueryOrderItemView struct {
	OrderID     int64  `json:"order_id"`
	Status      string `json:"status"`
	TotalAmount int64  `json:"total_amount"`
	Currency    string `json:"currency"`
	CreatedAt   int64  `json:"created_at"`
}

// listUserOrdersPayload 用户订单列表（第一页）。
type listUserOrdersPayload struct {
	Orders []QueryOrderItemView `json:"orders"`
}

// getOrderPayload 单笔订单查询成功体。
type getOrderPayload struct {
	Order QueryOrderItemView `json:"order"`
}
