package mq

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/cart/service"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

const TopicOrderStatusUpdate = "order_status_update"

type OrderStatusUpdateEvent struct {
	OrderID    int64       `json:"order_id"`
	UserID     int64       `json:"user_id,omitempty"`
	Status     OrderStatus `json:"status"`
	OrderKind  string      `json:"order_kind,omitempty"`
	ProductIDs []int64     `json:"product_ids,omitempty"`
	SKUIDs     []int64     `json:"sku_ids,omitempty"`
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
	cartService service.CartService
	logger      logger.LoggerV1
	consumer    *rocketmqx.Consumer
}

func NewOrderConsumer(
	client rmq_client.SimpleConsumer,
	cartService service.CartService,
	l logger.LoggerV1,
	options rocketmqx.ConsumerOptions,
) *OrderConsumer {
	c := &OrderConsumer{
		cartService: cartService,
		logger:      l,
	}
	c.consumer = rocketmqx.NewConsumer(client, rocketmqx.NewHandler[OrderStatusUpdateEvent](l, c.Consume), l, options)
	return c
}

func (c *OrderConsumer) Start() error {
	if err := c.consumer.Start(); err != nil {
		return err
	}
	c.logger.Info("cart order consumer started",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "cart-order-consumer"))
	return nil
}

func (c *OrderConsumer) Consume(_ *rmq_client.MessageView, evt OrderStatusUpdateEvent) error {
	if evt.Status != OrderStatusPaid || evt.OrderKind != "CART" {
		return nil
	}
	if evt.UserID == 0 || len(evt.SKUIDs) == 0 {
		c.logger.Warn("paid cart order event missing user or skus",
			logger.Int64("orderID", evt.OrderID))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.logger.Info("delete paid cart items",
		logger.Int64("orderID", evt.OrderID),
		logger.Int64("userID", evt.UserID),
		logger.Int("skuCount", len(evt.SKUIDs)))

	if err := c.cartService.DeleteItems(ctx, evt.UserID, evt.SKUIDs); err != nil {
		c.logger.Warn("delete cart items failed",
			logger.Int64("orderID", evt.OrderID),
			logger.Int64("userID", evt.UserID),
			logger.Error(err))
	}
	return nil
}

func (c *OrderConsumer) Stop() error {
	return c.consumer.Stop()
}
