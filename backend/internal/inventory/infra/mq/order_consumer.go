package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
)

const TopicOrderStatusUpdate = "order_status_update"

const (
	TopicDeadLetterStock = "inventory_dead_letter_stock"
	TopicDeadLetterOrder = "inventory_dead_letter_order"
	maxRetries           = 3
)

type OrderStatusUpdateEvent struct {
	OrderID int64       `json:"order_id"`
	Status  OrderStatus `json:"status"`
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
	client         sarama.Client
	producer       sarama.SyncProducer
	releaseStockUC *usecase.ReleaseStockUseCase
	refundStockUC  *usecase.RefundStockUseCase
	orderCli       orderservice.Client
	logger         logger.LoggerV1
	consumerGrp    sarama.ConsumerGroup
}

func NewOrderConsumer(
	client sarama.Client,
	producer sarama.SyncProducer,
	releaseUC *usecase.ReleaseStockUseCase,
	refundUC *usecase.RefundStockUseCase,
	orderCli orderservice.Client,
	l logger.LoggerV1,
) *OrderConsumer {
	return &OrderConsumer{
		client:         client,
		producer:       producer,
		releaseStockUC: releaseUC,
		refundStockUC:  refundUC,
		orderCli:       orderCli,
		logger:         l,
	}
}

func (c *OrderConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("inventory-consumer", c.client)
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
				c.logger.Error("order consumer exited", logger.Error(err))
			}
		}
	}()

	c.logger.Info("OrderConsumer started",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "inventory-consumer"))
	return nil
}

func (c *OrderConsumer) Consume(_ *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
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

func (c *OrderConsumer) handlePaid(ctx context.Context, evt OrderStatusUpdateEvent) {
	c.logger.Info("paid event received, skip inventory commit", logger.Int64("orderID", evt.OrderID))
}

func (c *OrderConsumer) handleCanceled(ctx context.Context, evt OrderStatusUpdateEvent) {
	releaseCmd := usecase.ReleaseStockCommand{
		OperationID: buildOrderOperationID(evt.OrderID, "release"),
	}
	if err := c.releaseStockUC.Execute(ctx, releaseCmd); err != nil {
		c.logger.Warn("release reserved stock failed on cancel",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
	}

	refundCmd := usecase.RefundStockCommand{
		OperationID: buildOrderOperationID(evt.OrderID, "refund"),
	}
	err := c.executeWithRetry(ctx, "RefundStockOnCancel", evt, func() error {
		return c.refundStockUC.Execute(ctx, refundCmd)
	})
	if errors.Is(err, domain.ErrDuplicateOperation) {
		c.logger.Info("RefundStockOnCancel duplicate", logger.Int64("orderID", evt.OrderID))
		return
	}
	if err != nil {
		return
	}

	c.logger.Info("order canceled, stock released/restored", logger.Int64("orderID", evt.OrderID))
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
	c.sendToStockDeadLetter(evt, opName, lastErr)
	return lastErr
}

func (c *OrderConsumer) asyncUpdateOrderStatus(ctx context.Context, evt OrderStatusUpdateEvent, action orderv1.ChangeOrderAction) {
	go func() {
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
			}

			_, lastErr = c.orderCli.ChangeOrderStatus(ctx, &orderv1.ChangeOrderStatusReq{
				OrderId: evt.OrderID,
				Action:  action,
			})
			if lastErr == nil {
				return
			}

			c.logger.Warn("retry order status update",
				logger.Int64("orderID", evt.OrderID),
				logger.Int("attempt", attempt+1),
				logger.Error(lastErr))
		}

		c.logger.Error("order status update retries exhausted", logger.Int64("orderID", evt.OrderID), logger.Error(lastErr))
		c.sendToOrderDeadLetter(evt, action, lastErr)
	}()
}

func (c *OrderConsumer) sendToStockDeadLetter(evt OrderStatusUpdateEvent, opName string, lastErr error) {
	msg := map[string]interface{}{
		"event":     evt,
		"operation": opName,
		"error":     lastErr.Error(),
		"time":      time.Now(),
	}
	msgBytes, _ := json.Marshal(msg)
	_, _, err := c.producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicDeadLetterStock,
		Key:   sarama.StringEncoder(fmt.Sprintf("%d", evt.OrderID)),
		Value: sarama.ByteEncoder(msgBytes),
	})
	if err != nil {
		c.logger.Error("send stock dead letter failed", logger.Int64("orderID", evt.OrderID), logger.Error(err))
	}
}

func (c *OrderConsumer) sendToOrderDeadLetter(evt OrderStatusUpdateEvent, action orderv1.ChangeOrderAction, lastErr error) {
	msg := map[string]interface{}{
		"event":  evt,
		"action": action,
		"error":  lastErr.Error(),
		"time":   time.Now(),
	}
	msgBytes, _ := json.Marshal(msg)
	_, _, err := c.producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicDeadLetterOrder,
		Key:   sarama.StringEncoder(fmt.Sprintf("%d", evt.OrderID)),
		Value: sarama.ByteEncoder(msgBytes),
	})
	if err != nil {
		c.logger.Error("send order dead letter failed", logger.Int64("orderID", evt.OrderID), logger.Error(err))
	}
}

func (c *OrderConsumer) Stop() error {
	if c.consumerGrp != nil {
		return c.consumerGrp.Close()
	}
	return nil
}
