package mq

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	pushconsumer "github.com/apache/rocketmq-client-go/v2/consumer"
	pushprimitive "github.com/apache/rocketmq-client-go/v2/primitive"
)

type OrderStatusUpdateEvent struct {
	OrderID   int64               `json:"order_id"`
	Status    orderv1.OrderStatus `json:"status"`
	OrderKind string              `json:"order_kind,omitempty"`
}

type OrderStatusConsumer struct {
	requestRepo seckilldomain.RequestRepository
	cache       seckilldomain.Cache
	soldOut     seckilldomain.SoldOutMarker
	logger      logger.LoggerV1
	consumer    pushConsumer
	options     SeckillConsumerOptions
}

func NewOrderStatusConsumer(
	consumer pushConsumer,
	requestRepo seckilldomain.RequestRepository,
	_ seckilldomain.ActivityRepository,
	cache seckilldomain.Cache,
	soldOut seckilldomain.SoldOutMarker,
	l logger.LoggerV1,
	options SeckillConsumerOptions,
) *OrderStatusConsumer {
	options = options.withDefaults()
	if soldOut == nil {
		soldOut = seckilldomain.NewNopSoldOutMarker()
	}
	c := &OrderStatusConsumer{
		requestRepo: requestRepo,
		cache:       cache,
		soldOut:     soldOut,
		logger:      l,
		consumer:    consumer,
		options:     options,
	}
	if c.consumer != nil {
		if err := c.subscribe(); err != nil {
			panic(err)
		}
	}
	return c
}

func (c *OrderStatusConsumer) subscribe() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Subscribe(TopicOrderStatusUpdate, pushconsumer.MessageSelector{
		Type:       pushconsumer.TAG,
		Expression: "*",
	}, c.consumeMessages)
}

func (c *OrderStatusConsumer) Start() error {
	if c.consumer == nil {
		return nil
	}
	if err := c.consumer.Start(); err != nil {
		return err
	}
	c.logger.Info("seckill order-status push consumer started",
		logger.Int("consumeConcurrency", c.options.GlobalWorkerNum),
		logger.Field{Key: "handleTimeout", Value: c.options.HandleTimeout})
	return nil
}

func (c *OrderStatusConsumer) Stop() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Shutdown()
}

func (c *OrderStatusConsumer) consumeMessages(ctx context.Context, msgs ...*pushprimitive.MessageExt) (pushconsumer.ConsumeResult, error) {
	for _, msg := range msgs {
		var evt OrderStatusUpdateEvent
		if err := json.Unmarshal(msg.Body, &evt); err != nil {
			c.logger.Error("decode order-status message failed, skip poison message",
				logger.Error(err),
				logger.String("messageID", msg.MsgId))
			continue
		}
		if err := c.handleEventWithTimeout(ctx, evt); err != nil {
			c.logger.Warn("process order-status message failed, waiting for MQ retry",
				logger.Error(err),
				logger.String("messageID", msg.MsgId),
				logger.Int64("orderID", evt.OrderID),
				logger.Int32("reconsumeTimes", msg.ReconsumeTimes))
			return pushconsumer.ConsumeRetryLater, nil
		}
	}
	return pushconsumer.ConsumeSuccess, nil
}

func (c *OrderStatusConsumer) handleEventWithTimeout(ctx context.Context, evt OrderStatusUpdateEvent) error {
	taskCtx := ctx
	cancel := func() {}
	if c.options.HandleTimeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, c.options.HandleTimeout)
	}
	defer cancel()
	return c.consume(taskCtx, evt)
}

func (c *OrderStatusConsumer) consume(ctx context.Context, evt OrderStatusUpdateEvent) error {
	kind := strings.ToUpper(strings.TrimSpace(evt.OrderKind))
	if kind != "" && kind != "SECKILL" {
		return nil
	}

	switch evt.Status {
	case orderv1.OrderStatus_ORDER_STATUS_PAID:
		return nil
	case orderv1.OrderStatus_ORDER_STATUS_CANCELED, orderv1.OrderStatus_ORDER_STATUS_REFUNDED:
		return c.onOrderClosed(ctx, evt)
	default:
		return nil
	}
}

func (c *OrderStatusConsumer) onOrderClosed(ctx context.Context, evt OrderStatusUpdateEvent) error {
	requestNo := strconv.FormatInt(evt.OrderID, 10)
	req, changed, err := c.requestRepo.CloseByOrderResult(ctx, requestNo, failReasonForOrderStatus(evt.Status))
	if err != nil {
		if errors.Is(err, seckilldomain.ErrRequestNotFound) {
			return nil
		}
		return err
	}
	if !changed {
		return nil
	}

	if err = c.cache.Compensate(ctx, req.ActivityID, req.UserID, req.RequestNo, seckilldomain.Result{
		RequestNo:  req.RequestNo,
		Status:     seckilldomain.RequestStatusFailed,
		FailReason: req.FailReason,
	}); err != nil {
		return err
	}
	// 订单取消/退款后库存会回补，本机售罄标记也要清掉。
	c.soldOut.Clear(req.ActivityID)
	c.logger.Info("order closed, clear local sold-out flag",
		logger.String("requestNo", req.RequestNo),
		logger.Int64("activityID", req.ActivityID),
		logger.Int64("userID", req.UserID),
		logger.String("failReason", req.FailReason))
	return nil
}

func failReasonForOrderStatus(status orderv1.OrderStatus) string {
	if status == orderv1.OrderStatus_ORDER_STATUS_REFUNDED {
		return seckilldomain.FailReasonOrderRefunded
	}
	return seckilldomain.FailReasonOrderCanceled
}
