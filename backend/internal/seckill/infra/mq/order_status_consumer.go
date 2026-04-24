package mq

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/IBM/sarama"
	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
)

type OrderStatusUpdateEvent struct {
	OrderID   int64               `json:"order_id"`
	Status    orderv1.OrderStatus `json:"status"`
	OrderKind string              `json:"order_kind,omitempty"`
}

type OrderStatusConsumer struct {
	client       sarama.Client
	requestRepo  seckilldomain.RequestRepository
	activityRepo seckilldomain.ActivityRepository
	cache        seckilldomain.Cache
	logger       logger.LoggerV1
	consumerGrp  sarama.ConsumerGroup
}

func NewOrderStatusConsumer(client sarama.Client, requestRepo seckilldomain.RequestRepository, activityRepo seckilldomain.ActivityRepository, cache seckilldomain.Cache, l logger.LoggerV1) *OrderStatusConsumer {
	return &OrderStatusConsumer{
		client:       client,
		requestRepo:  requestRepo,
		activityRepo: activityRepo,
		cache:        cache,
		logger:       l,
	}
}

func (c *OrderStatusConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("seckill-order-status-consumer", c.client)
	if err != nil {
		return err
	}
	c.consumerGrp = cg

	go func() {
		for {
			err := cg.Consume(context.Background(), []string{TopicOrderStatusUpdate}, saramax.NewHandler[OrderStatusUpdateEvent](c.logger, c.consume))
			if err != nil {
				c.logger.Error("seckill order status consumer exited", logger.Error(err))
			}
		}
	}()
	return nil
}

func (c *OrderStatusConsumer) consume(_ *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
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
	req, err := c.requestRepo.FindByRequestNo(ctx, requestNo)
	if err != nil {
		if errors.Is(err, seckilldomain.ErrRequestNotFound) {
			return nil
		}
		return err
	}
	if req.Status != seckilldomain.RequestStatusProcessing &&
		req.Status != seckilldomain.RequestStatusQualified &&
		req.Status != seckilldomain.RequestStatusLegacySuccess {
		return nil
	}

	total := req.Quantity
	if total <= 0 {
		total = 1
	}

	action := "cancel"
	failReason := seckilldomain.FailReasonOrderCanceled
	if evt.Status == orderv1.OrderStatus_ORDER_STATUS_REFUNDED {
		action = "refund"
		failReason = seckilldomain.FailReasonOrderRefunded
	}

	if err := c.activityRepo.IncreaseStock(ctx, req.ActivityID, fmt.Sprintf("order_%d_%s", evt.OrderID, action), total); err != nil {
		return err
	}
	if err := c.activityRepo.DeleteSuccessClaim(ctx, req.ActivityID, req.UserID); err != nil {
		return err
	}

	n, err := c.requestRepo.MarkFailByRequestNoIfActive(ctx, requestNo, failReason)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}

	if err := c.cache.Compensate(ctx, req.ActivityID, req.UserID, total, true); err != nil {
		return err
	}
	return c.cache.SetResult(ctx, seckilldomain.Result{
		RequestNo:  req.RequestNo,
		Status:     seckilldomain.RequestStatusFail,
		FailReason: failReason,
	})
}

func (c *OrderStatusConsumer) Stop() error {
	if c.consumerGrp != nil {
		return c.consumerGrp.Close()
	}
	return nil
}
