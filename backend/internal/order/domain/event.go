package domain

type OrderStatusUpdateEvent struct {
	OrderID int64            `json:"order_id"`
	Status  OrderStatus      `json:"status"`
	Items   []OrderEventItem `json:"items,omitempty"` // CommitStock需要，其他操作可为空
}

// OrderEventItem 事件中的商品信息（精简版，只包含库存需要的字段）
type OrderEventItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

// 事件ID和负载
type OutboxEvent struct {
	ID    int64                  `json:"id"`
	Event OrderStatusUpdateEvent `json:"event"`
}
