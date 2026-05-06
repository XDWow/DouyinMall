package mq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

const TopicOrderStatusUpdate = "order_status_update"

const (
	maxRetries = 3
)

type OrderStatusUpdateEvent struct {
	OrderID   int64       `json:"order_id"`
	Status    OrderStatus `json:"status"`
	OrderKind string      `json:"order_kind,omitempty"`
}

type OrderStatus = orderv1.OrderStatus

const (
	OrderStatusUnknown   = orderv1.OrderStatus_ORDER_STATUS_UNKNOWN
	OrderStatusCreated   = orderv1.OrderStatus_ORDER_STATUS_CREATED
	OrderStatusPaid      = orderv1.OrderStatus_ORDER_STATUS_PAID
	OrderStatusShipped   = orderv1.OrderStatus_ORDER_STATUS_SHIPPED
	OrderStatusCompleted = orderv1.OrderStatus_ORDER_STATUS_COMPLETED
	OrderStatusCanceled  = orderv1.OrderStatus_ORDER_STATUS_CANCELED
	OrderStatusRefunded  = orderv1.OrderStatus_ORDER_STATUS_REFUNDED
)

type OrderConsumer struct {
	refundStockUC *usecase.RefundStockUseCase
	logger        logger.LoggerV1
	consumer      *rocketmqx.Consumer
}

func NewOrderConsumer(
	client rmq_client.SimpleConsumer,
	refundUC *usecase.RefundStockUseCase,
	l logger.LoggerV1,
	options rocketmqx.ConsumerOptions,
) *OrderConsumer {
	c := &OrderConsumer{
		refundStockUC: refundUC,
		logger:        l,
	}
	if client != nil {
		c.consumer = rocketmqx.NewConsumer(client, rocketmqx.NewHandler[OrderStatusUpdateEvent](l, c.Consume), l, options)
	}
	return c
}

func (c *OrderConsumer) Start() error {
	if c.consumer == nil {
		return nil
	}
	if err := c.consumer.Start(); err != nil {
		return err
	}
	c.logger.Info("inventory order consumer started",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "inventory-consumer"))
	return nil
}

func (c *OrderConsumer) Consume(_ *rmq_client.MessageView, evt OrderStatusUpdateEvent) error {
	kind := normalizeOrderKind(evt.OrderKind)
	switch kind {
	case "SECKILL":
		return nil
	case "", "DIRECT_BUY", "CART":
	default:
		c.logger.Warn("skip unsupported order kind",
			logger.Int64("orderID", evt.OrderID),
			logger.String("orderKind", evt.OrderKind),
		)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch evt.Status {
	case OrderStatusPaid:
		c.handlePaid(ctx, evt)
	case OrderStatusCanceled:
		c.handleCanceled(ctx, evt)
	case OrderStatusRefunded:
		c.handleRefunded(ctx, evt)
	}

	return nil
}

func (c *OrderConsumer) handlePaid(_ context.Context, evt OrderStatusUpdateEvent) {
	c.logger.Info("paid event received, skip inventory commit", logger.Int64("orderID", evt.OrderID))
}

func (c *OrderConsumer) handleCanceled(ctx context.Context, evt OrderStatusUpdateEvent) {
	cmd := usecase.RefundStockCommand{
		OperationID: buildOrderOperationID(evt.OrderID, "refund"),
	}
	err := c.executeWithRetry(ctx, "RefundStockOnCancel", evt, func() error {
		return c.refundStockUC.Execute(ctx, cmd)
	})
	if errors.Is(err, domain.ErrDuplicateOperation) {
		c.logger.Info("RefundStockOnCancel duplicate", logger.Int64("orderID", evt.OrderID))
		return
	}
	if err != nil {
		return
	}

	c.logger.Info("order canceled, stock restored", logger.Int64("orderID", evt.OrderID))
}

func (c *OrderConsumer) handleRefunded(ctx context.Context, evt OrderStatusUpdateEvent) {
	cmd := usecase.RefundStockCommand{
		OperationID: buildOrderOperationID(evt.OrderID, "refund"),
	}
	err := c.executeWithRetry(ctx, "RefundStock", evt, func() error {
		return c.refundStockUC.Execute(ctx, cmd)
	})
	if errors.Is(err, domain.ErrDuplicateOperation) {
		c.logger.Info("RefundStock duplicate", logger.Int64("orderID", evt.OrderID))
		return
	}
	if err != nil {
		return
	}

	c.logger.Info("RefundStock success", logger.Int64("orderID", evt.OrderID))
}

func buildOrderOperationID(orderID int64, action string) string {
	return fmt.Sprintf("order_%d_%s", orderID, action)
}

func normalizeOrderKind(orderKind string) string {
	return strings.ToUpper(strings.TrimSpace(orderKind))
}

func (c *OrderConsumer) executeWithRetry(ctx context.Context, opName string, evt OrderStatusUpdateEvent, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, domain.ErrDuplicateOperation) || errors.Is(lastErr, domain.ErrInsufficientStock) {
			return lastErr
		}

		c.logger.Warn(opName+" retry",
			logger.Int64("orderID", evt.OrderID),
			logger.Int("attempt", attempt+1),
			logger.Error(lastErr))
	}

	c.logger.Error(opName+" retries exhausted", logger.Int64("orderID", evt.OrderID), logger.Error(lastErr))
	return lastErr
}

func (c *OrderConsumer) Stop() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Stop()
}
