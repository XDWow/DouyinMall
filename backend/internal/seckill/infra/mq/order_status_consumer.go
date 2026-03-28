package mq

import (
	"context"
	"errors"
	"fmt"

	"github.com/IBM/sarama"
	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
)

type OrderStatusUpdateEvent struct {
	OrderID int64               `json:"order_id"`
	Status  orderv1.OrderStatus `json:"status"`
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
				c.logger.Error("秒杀订单状态消费者退出", logger.Error(err))
			}
		}
	}()
	return nil
}

func (c *OrderStatusConsumer) consume(_ *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
	if evt.Status != orderv1.OrderStatus_ORDER_STATUS_CANCELED &&
		evt.Status != orderv1.OrderStatus_ORDER_STATUS_REFUNDED {
		return nil
	}

	req, err := c.requestRepo.FindByOrderID(context.Background(), evt.OrderID)
	if err != nil {
		if errors.Is(err, seckilldomain.ErrRequestNotFound) {
			return nil
		}
		return err
	}

	total := req.Quantity
	if total <= 0 {
		total = 1
	}

	action := "cancel"
	if evt.Status == orderv1.OrderStatus_ORDER_STATUS_REFUNDED {
		action = "refund"
	}

	if err := c.activityRepo.IncreaseStock(context.Background(), req.ActivityID, fmt.Sprintf("order_%d_%s", evt.OrderID, action), total); err != nil {
		return err
	}
	return c.cache.IncreaseStock(context.Background(), req.ActivityID, total)
}

func (c *OrderStatusConsumer) Stop() error {
	if c.consumerGrp != nil {
		return c.consumerGrp.Close()
	}
	return nil
}
