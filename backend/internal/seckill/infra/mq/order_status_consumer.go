package mq

import (
	"context"
	"errors"
	"strconv"
	"strings"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type OrderStatusUpdateEvent struct {
	OrderID   int64               `json:"order_id"`
	Status    orderv1.OrderStatus `json:"status"`
	OrderKind string              `json:"order_kind,omitempty"`
}

type OrderStatusConsumer struct {
	requestRepo seckilldomain.RequestRepository
	cache       seckilldomain.Cache
	logger      logger.LoggerV1
	consumer    *rocketmqx.Consumer
}

func NewOrderStatusConsumer(
	client rmq_client.SimpleConsumer,
	requestRepo seckilldomain.RequestRepository,
	_ seckilldomain.ActivityRepository,
	cache seckilldomain.Cache,
	l logger.LoggerV1,
	options rocketmqx.ConsumerOptions,
) *OrderStatusConsumer {
	c := &OrderStatusConsumer{
		requestRepo: requestRepo,
		cache:       cache,
		logger:      l,
	}
	if client != nil {
		c.consumer = rocketmqx.NewConsumer(client, rocketmqx.NewHandler[OrderStatusUpdateEvent](l, c.consume), l, options)
	}
	return c
}

func (c *OrderStatusConsumer) Start() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Start()
}

func (c *OrderStatusConsumer) consume(_ *rmq_client.MessageView, evt OrderStatusUpdateEvent) error {
	kind := strings.ToUpper(strings.TrimSpace(evt.OrderKind))
	if kind != "" && kind != "SECKILL" {
		return nil
	}

	ctx := context.Background()
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

	return c.cache.Compensate(ctx, req.ActivityID, req.UserID, req.RequestNo, seckilldomain.Result{
		RequestNo:  req.RequestNo,
		Status:     seckilldomain.RequestStatusFailed,
		FailReason: req.FailReason,
	})
}

func failReasonForOrderStatus(status orderv1.OrderStatus) string {
	if status == orderv1.OrderStatus_ORDER_STATUS_REFUNDED {
		return seckilldomain.FailReasonOrderRefunded
	}
	return seckilldomain.FailReasonOrderCanceled
}

func (c *OrderStatusConsumer) Stop() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Stop()
}
