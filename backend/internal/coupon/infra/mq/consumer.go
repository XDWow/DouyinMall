package mq

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

const TopicOrderStatusUpdate = "order_status_update"

type OrderStatusUpdateEvent struct {
	OrderID int64       `json:"order_id"`
	Status  OrderStatus `json:"status"`
}

type OrderStatus uint8

const (
	OrderStatusUnknown OrderStatus = iota
	OrderStatusCreated
	OrderStatusPaid
	OrderStatusShipped
	OrderStatusCompleted
	OrderStatusCanceled
	OrderStatusRefunded
)

type OrderConsumer struct {
	commitUC  *usecase.CommitCouponUseCase
	releaseUC *usecase.ReleaseCouponUseCase
	refundUC  *usecase.RefundCouponUseCase
	logger    logger.LoggerV1
	consumer  *rocketmqx.Consumer
}

func NewOrderConsumer(
	client rmq_client.SimpleConsumer,
	commitUC *usecase.CommitCouponUseCase,
	releaseUC *usecase.ReleaseCouponUseCase,
	refundUC *usecase.RefundCouponUseCase,
	l logger.LoggerV1,
	options rocketmqx.ConsumerOptions,
) *OrderConsumer {
	c := &OrderConsumer{
		commitUC:  commitUC,
		releaseUC: releaseUC,
		refundUC:  refundUC,
		logger:    l,
	}
	c.consumer = rocketmqx.NewConsumer(client, rocketmqx.NewHandler[OrderStatusUpdateEvent](l, c.Consume), l, options)
	return c
}

func (c *OrderConsumer) Start() error {
	if err := c.consumer.Start(); err != nil {
		return err
	}
	c.logger.Info("coupon order consumer started",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "coupon-consumer"))
	return nil
}

func (c *OrderConsumer) Consume(msg *rmq_client.MessageView, evt OrderStatusUpdateEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.logger.Info("coupon order status event received",
		logger.Int64("orderID", evt.OrderID),
		logger.Int("status", int(evt.Status)),
		logger.String("topic", msg.GetTopic()),
		logger.String("messageID", msg.GetMessageId()),
		logger.Int64("offset", msg.GetOffset()))

	switch evt.Status {
	case OrderStatusPaid:
		return c.handlePaid(ctx, evt)
	case OrderStatusCanceled:
		return c.handleCanceled(ctx, evt)
	case OrderStatusRefunded:
		return c.handleRefunded(ctx, evt)
	default:
		return nil
	}
}

func (c *OrderConsumer) handlePaid(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.commitUC.Execute(ctx, usecase.CommitCouponInput{OrderID: evt.OrderID})
	if err != nil {
		c.logger.Error("commit coupon failed",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err),
		)
		return err
	}

	c.logger.Info("coupon committed", logger.Int64("orderID", evt.OrderID))
	return nil
}

func (c *OrderConsumer) handleCanceled(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.releaseUC.Execute(ctx, usecase.ReleaseCouponInput{OrderID: evt.OrderID})
	if err != nil {
		c.logger.Error("release coupon failed",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err),
		)
		return err
	}

	c.logger.Info("coupon released", logger.Int64("orderID", evt.OrderID))
	return nil
}

func (c *OrderConsumer) handleRefunded(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.refundUC.Execute(ctx, usecase.RefundCouponInput{OrderID: evt.OrderID})
	if err != nil {
		c.logger.Error("refund coupon failed",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err),
		)
		return err
	}

	c.logger.Info("coupon refunded", logger.Int64("orderID", evt.OrderID))
	return nil
}

func (c *OrderConsumer) Stop() error {
	return c.consumer.Stop()
}
