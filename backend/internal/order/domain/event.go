package domain

const EventTypeOrderStatusChanged = "order.status.changed"

type OrderStatusUpdateEvent struct {
	OrderID    int64       `json:"order_id"`
	UserID     int64       `json:"user_id,omitempty"`
	Status     OrderStatus `json:"status"`
	OrderKind  string      `json:"order_kind,omitempty"`
	ProductIDs []int64     `json:"product_ids,omitempty"`
}

type OutboxEvent struct {
	ID    int64                  `json:"id"`
	Event OrderStatusUpdateEvent `json:"event"`
}

func BuildOrderStatusUpdateEvent(order *Order) OrderStatusUpdateEvent {
	event := OrderStatusUpdateEvent{
		OrderID:   order.ID,
		Status:    order.Status,
		OrderKind: order.OrderKind,
	}
	if order.Status != OrderStatusPaid || order.OrderKind != OrderKindCart {
		return event
	}

	event.UserID = order.UserID
	event.ProductIDs = make([]int64, 0, len(order.OrderItems))
	for _, item := range order.OrderItems {
		event.ProductIDs = append(event.ProductIDs, item.ProductID)
	}
	return event
}


