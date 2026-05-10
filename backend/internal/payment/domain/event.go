package domain

// 支付状态变化通过支付 outbox 投递，订单服务异步消费后推进状态机。
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
