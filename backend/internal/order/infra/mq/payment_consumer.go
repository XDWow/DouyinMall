package mq

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

const TopicPaymentStatusUpdate = "payment_status_update"

type PaymentStatus uint8

const (
	PaymentStatusUnknown PaymentStatus = iota
	PaymentStatusInit
	PaymentStatusSuccess
	PaymentStatusFailed
	PaymentStatusRefund
)

type PaymentStatusUpdateEvent struct {
	BizTradeNo string        `json:"biz_trade_no"`
	OrderID    int64         `json:"order_id"`
	Status     PaymentStatus `json:"status"`
	TxnID      string        `json:"txn_id,omitempty"`
}

type OrderStatusChanger interface {
	ChangeOrderStatus(ctx context.Context, orderID int64, action domain.OrderAction) error
}

type PaymentStatusConsumer struct {
	orderStatusChanger OrderStatusChanger
	logger             logger.LoggerV1
	consumer           *rocketmqx.Consumer
}

func NewPaymentStatusConsumer(
	client rmq_client.SimpleConsumer,
	orderStatusChanger OrderStatusChanger,
	l logger.LoggerV1,
	options rocketmqx.ConsumerOptions,
) *PaymentStatusConsumer {
	c := &PaymentStatusConsumer{
		orderStatusChanger: orderStatusChanger,
		logger:             l,
	}
	c.consumer = rocketmqx.NewConsumer(client, rocketmqx.NewHandler[PaymentStatusUpdateEvent](l, c.Consume), l, options)
	return c
}

func (c *PaymentStatusConsumer) Start() error {
	if err := c.consumer.Start(); err != nil {
		return err
	}
	c.logger.Info("order payment status consumer started",
		logger.String("topic", TopicPaymentStatusUpdate),
		logger.String("consumerGroup", "order-payment-consumer"))
	return nil
}

func (c *PaymentStatusConsumer) Stop() error {
	return c.consumer.Stop()
}

func (c *PaymentStatusConsumer) Consume(_ *rmq_client.MessageView, evt PaymentStatusUpdateEvent) error {
	action, ok := paymentStatusToOrderAction(evt.Status)
	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.orderStatusChanger.ChangeOrderStatus(ctx, evt.OrderID, action)
	if err == nil {
		return nil
	}

	if errors.Is(err, domain.ErrInvalidStatusTransition) {
		// Retrying cannot make a canceled/refunded order payable; log and leave refund/ops flow to handle it.
		c.logger.Error("payment status cannot be applied to current order status",
			logger.Error(err),
			logger.Int64("orderID", evt.OrderID),
			logger.Int("paymentStatus", int(evt.Status)))
		return nil
	}
	return err
}

func paymentStatusToOrderAction(status PaymentStatus) (domain.OrderAction, bool) {
	switch status {
	case PaymentStatusSuccess:
		return domain.OrderActionPay, true
	case PaymentStatusRefund:
		return domain.OrderActionRefund, true
	default:
		return domain.OrderActionUnknown, false
	}
}
