package domain

// Payment status changes are published through payment outbox.
// Order service consumes this event to advance CREATED -> PAID asynchronously.
const EventTypePaymentStatusChanged = "payment.status.changed"

type PaymentStatusUpdateEvent struct {
	BizTradeNo string        `json:"biz_trade_no"`
	OrderID    int64         `json:"order_id"`
	Status     PaymentStatus `json:"status"`
	TxnID      string        `json:"txn_id,omitempty"`
}

type PaymentOutboxEvent struct {
	ID    int64                    `json:"id"`
	Event PaymentStatusUpdateEvent `json:"event"`
}
