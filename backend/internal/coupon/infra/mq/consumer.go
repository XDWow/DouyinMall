package mq

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
)

const TopicOrderStatusUpdate = "order_status_update"

// OrderStatusUpdateEvent 订单状态变更事件
type OrderStatusUpdateEvent struct {
	OrderID int64       `json:"order_id"`
	Status  OrderStatus `json:"status"`
}

type OrderStatus uint8

const (
	OrderStatusUnknown   OrderStatus = iota
	OrderStatusCreated               // 1: 待支付
	OrderStatusPaid                  // 2: 已支付（需要确认优惠券）
	OrderStatusToShip                // 3: 待发货
	OrderStatusShipped               // 4: 已发货
	OrderStatusCompleted             // 5: 已完成
	OrderStatusCanceled              // 6: 已取消（需要释放优惠券）
	OrderStatusRefunded              // 7: 已退款（需要退还优惠券）
)

/*
OrderConsumer 订单状态变更消费者

职责：
1. 监听订单状态变更消息
2. 根据订单状态执行优惠券操作：
  - 支付成功(Paid) → 确认使用优惠券（Locked → Used）
  - 订单取消(Canceled) → 释放优惠券（Locked → Unused）
  - 订单退款(Refunded) → 退还优惠券（Used → Unused）

设计原则：
- 幂等性：利用UpdateStatusByOrderID的条件更新保证幂等
- 容错性：使用本地重试 + 统一ACK（saramax.Handler内置3次重试）
- 解耦：只负责消息路由，业务逻辑在UseCase层
*/
type OrderConsumer struct {
	client    sarama.Client
	commitUC  *usecase.CommitCouponUseCase  // 确认使用
	releaseUC *usecase.ReleaseCouponUseCase // 释放（取消）
	refundUC  *usecase.RefundCouponUseCase  // 退还（退款）
	logger    logger.LoggerV1
}

func NewOrderConsumer(
	client sarama.Client,
	commitUC *usecase.CommitCouponUseCase,
	releaseUC *usecase.ReleaseCouponUseCase,
	refundUC *usecase.RefundCouponUseCase,
	l logger.LoggerV1,
) *OrderConsumer {
	return &OrderConsumer{
		client:    client,
		commitUC:  commitUC,
		releaseUC: releaseUC,
		refundUC:  refundUC,
		logger:    l,
	}
}

func (c *OrderConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("coupon-consumer", c.client)
	if err != nil {
		return err
	}

	go func() {
		for {
			err := cg.Consume(
				context.Background(),
				[]string{TopicOrderStatusUpdate},
				saramax.NewHandler[OrderStatusUpdateEvent](c.logger, c.Consume),
			)
			if err != nil {
				c.logger.Error("优惠券消费者异常退出", logger.Error(err))
				// 短暂等待后重试，避免疯狂重连
				time.Sleep(time.Second)
			}
		}
	}()

	c.logger.Info("OrderConsumer已启动",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "coupon-consumer"))

	return nil
}

// Consume 消息路由：根据状态分发到不同处理方法
// 返回nil表示ACK，返回error会触发saramax.Handler的重试逻辑（最多3次）
func (c *OrderConsumer) Consume(msg *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.logger.Info("收到订单状态变更消息",
		logger.Int64("orderID", evt.OrderID),
		logger.Int("status", int(evt.Status)),
		logger.String("topic", msg.Topic),
		logger.Int64("partition", int64(msg.Partition)),
		logger.Int64("offset", msg.Offset))

	switch evt.Status {
	case OrderStatusPaid:
		return c.handlePaid(ctx, evt)
	case OrderStatusCanceled:
		return c.handleCanceled(ctx, evt)
	case OrderStatusRefunded:
		return c.handleRefunded(ctx, evt)
	default:
		// 其他状态不需要处理，直接ACK
		return nil
	}
}

// 订单支付成功 → 确认使用优惠券（Locked → Used）
func (c *OrderConsumer) handlePaid(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.commitUC.Execute(ctx, usecase.CommitCouponInput{
		OrderID: evt.OrderID,
	})
	if err != nil {
		c.logger.Error("确认使用优惠券失败",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
		return err // 返回error触发重试
	}

	c.logger.Info("确认使用优惠券成功",
		logger.Int64("orderID", evt.OrderID))
	return nil
}

// 订单取消 → 释放优惠券（Locked → Unused）
func (c *OrderConsumer) handleCanceled(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.releaseUC.Execute(ctx, usecase.ReleaseCouponInput{
		OrderID: evt.OrderID,
	})
	if err != nil {
		c.logger.Error("释放优惠券失败",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
		return err // 返回error触发重试
	}

	c.logger.Info("释放优惠券成功",
		logger.Int64("orderID", evt.OrderID))
	return nil
}

// 订单退款 → 退还优惠券（Used → Unused）
func (c *OrderConsumer) handleRefunded(ctx context.Context, evt OrderStatusUpdateEvent) error {
	err := c.refundUC.Execute(ctx, usecase.RefundCouponInput{
		OrderID: evt.OrderID,
	})
	if err != nil {
		c.logger.Error("退还优惠券失败",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
		return err // 返回error触发重试
	}

	c.logger.Info("退还优惠券成功",
		logger.Int64("orderID", evt.OrderID))
	return nil
}
