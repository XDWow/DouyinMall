package mq

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/cart/service"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
)

const TopicOrderStatusUpdate = "order_status_update"

type OrderStatusUpdateEvent struct {
	OrderID    int64       `json:"order_id"`
	UserID     int64       `json:"user_id,omitempty"`
	Status     OrderStatus `json:"status"`
	OrderKind  string      `json:"order_kind,omitempty"`
	ProductIDs []int64     `json:"product_ids,omitempty"`
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
	client      sarama.Client
	cartService service.CartService
	logger      logger.LoggerV1
	consumerGrp sarama.ConsumerGroup
}

func NewOrderConsumer(
	client sarama.Client,
	cartService service.CartService,
	l logger.LoggerV1,
) *OrderConsumer {
	return &OrderConsumer{
		client:      client,
		cartService: cartService,
		logger:      l,
	}
}

func (c *OrderConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("cart-order-consumer", c.client)
	if err != nil {
		return err
	}
	c.consumerGrp = cg

	go func() {
		for {
			err := cg.Consume(
				context.Background(),
				[]string{TopicOrderStatusUpdate},
				saramax.NewHandler[OrderStatusUpdateEvent](c.logger, c.Consume),
			)
			if err != nil {
				c.logger.Error("cart order consumer exited, retrying", logger.Error(err))
			}
		}
	}()

	c.logger.Info("Cart OrderConsumer started",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "cart-order-consumer"))
	return nil
}

func (c *OrderConsumer) Consume(_ *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
	if evt.Status != OrderStatusPaid || evt.OrderKind != "CART" {
		return nil
	}
	if evt.UserID == 0 || len(evt.ProductIDs) == 0 {
		c.logger.Warn("paid cart order event missing user or products",
			logger.Int64("orderID", evt.OrderID))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.logger.Info("delete paid cart items",
		logger.Int64("orderID", evt.OrderID),
		logger.Int64("userID", evt.UserID),
		logger.Int("productCount", len(evt.ProductIDs)))

	if err := c.cartService.DeleteItems(ctx, evt.UserID, evt.ProductIDs); err != nil {
		c.logger.Warn("delete cart items failed",
			logger.Int64("orderID", evt.OrderID),
			logger.Int64("userID", evt.UserID),
			logger.Error(err))
	}
	return nil
}

func (c *OrderConsumer) Stop() error {
	if c.consumerGrp != nil {
		return c.consumerGrp.Close()
	}
	return nil
}
