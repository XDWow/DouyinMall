package domain

type OrderStatusUpdateEvent struct {
	OrderID int64       `json:"order_id"`
	Status  OrderStatus `json:"status"`
}

// 事件ID和负载
type OutboxEvent struct {
	ID    int64                  `json:"id"`
	Event OrderStatusUpdateEvent `json:"event"`
}
