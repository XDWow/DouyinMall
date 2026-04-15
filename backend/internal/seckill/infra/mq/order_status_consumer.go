package mq

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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
				c.logger.Error("秒杀订单状态消费者消费出错", logger.Error(err))
			}
		}
	}()
	return nil
}

func (c *OrderStatusConsumer) consume(_ *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
	ctx := context.Background()
	switch evt.Status {
	case orderv1.OrderStatus_ORDER_STATUS_PAID:
		// 抢到资格后前端改查订单/支付；秒杀侧不再随 PAID 变更轮询状态。
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
	if req.Status != seckilldomain.RequestStatusProcessing && req.Status != seckilldomain.RequestStatusQualified && req.Status != seckilldomain.RequestStatusLegacySuccess {
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

	// 1) 活动库库存回补（operation_id 幂等，避免重复消息双补）
	if err := c.activityRepo.IncreaseStock(ctx, req.ActivityID, fmt.Sprintf("order_%d_%s", evt.OrderID, action), total); err != nil {
		return err
	}
	// 2) 删除秒杀成功占有（一人一单 DB）
	if err := c.activityRepo.DeleteSuccessClaim(ctx, req.ActivityID, req.UserID); err != nil {
		return err
	}
	// 3) 流水回退为失败（仅当仍绑定该订单且为待支付/已支付态时更新；0 行表示已处理过）
	n, err := c.requestRepo.MarkFailByRequestNoIfActive(ctx, requestNo, failReason)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	// 4) Redis：恢复提交阶段预扣库存 + 删除用户占位
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
