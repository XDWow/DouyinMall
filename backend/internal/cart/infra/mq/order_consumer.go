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

// OrderStatusUpdateEvent 只保留购物车清理所需字段
type OrderStatusUpdateEvent struct {
	OrderID int64            `json:"order_id"`
	UserID  int64            `json:"user_id,omitempty"`
	Status  OrderStatus      `json:"status"`
	Items   []OrderEventItem `json:"items,omitempty"`
}

type OrderEventItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type OrderStatus uint8

const (
	OrderStatusUnknown   OrderStatus = iota
	OrderStatusCreated               // 待支付
	OrderStatusPaid                  // 已支付
	OrderStatusToShip                // 库存已确认，待发货
	OrderStatusShipped               // 已发货
	OrderStatusCompleted             // 已完成
	OrderStatusCanceled              // 已取消
	OrderStatusRefunded              // 已退款
)

// OrderConsumer 监听订单状态变更事件，支付成功后清理购物车中已下单的商品
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
				c.logger.Error("购物车订单消费者退出，准备重连", logger.Error(err))
			}
		}
	}()

	c.logger.Info("Cart OrderConsumer已启动",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "cart-order-consumer"))

	return nil
}

// 只处理支付成功事件，清理购物车
func (c *OrderConsumer) Consume(msg *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
	if evt.Status != OrderStatusPaid {
		return nil // 其他状态不关心，直接ACK
	}
	if evt.UserID == 0 || len(evt.Items) == 0 {
		c.logger.Warn("支付成功事件缺少 UserID 或 Items，跳过购物车清理",
			logger.Int64("orderID", evt.OrderID))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.logger.Info("清理已支付订单购物车",
		logger.Int64("orderID", evt.OrderID),
		logger.Int64("userID", evt.UserID),
		logger.Int("itemCount", len(evt.Items)))

	productIDs := make([]int64, len(evt.Items))
	for i, item := range evt.Items {
		productIDs[i] = item.ProductID
	}
	if err := c.cartService.DeleteItems(ctx, evt.UserID, productIDs); err != nil {
		// 购物车清理失败不阻塞后续流程，记录日志即可
		c.logger.Warn("清理购物车商品失败",
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
