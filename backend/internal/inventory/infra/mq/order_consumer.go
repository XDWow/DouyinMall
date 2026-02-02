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

// 两个死信队列：库存操作失败 和 订单状态更新失败
const (
	TopicDeadLetterStock = "inventory_dead_letter_stock" // 库存操作失败
	TopicDeadLetterOrder = "inventory_dead_letter_order" // 更新订单状态失败
)
const maxRetries = 3 // 本地重试次数

type OrderStatusUpdateEvent struct {
	OrderID int64            `json:"order_id"`
	Status  OrderStatus      `json:"status"`
	Items   []OrderEventItem `json:"items,omitempty"` // CommitStock需要，其他操作可为空
}

// OrderEventItem 事件中的商品信息
type OrderEventItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type OrderStatus uint8

func (s OrderStatus) AsUint8() uint8 {
	return uint8(s)
}

const (
	OrderStatusUnknown   OrderStatus = iota
	OrderStatusCreated               // 待支付
	OrderStatusPaid                  // 已支付
	OrderStatusToShip                // 库存已确认，待发货
	OrderStatusShipped               // 已发货
	OrderStatusCompleted             // 已完成（确认收货）
	OrderStatusCanceled              // 已取消（未支付超时或者库存确认失败）
	OrderStatusRefunded              // 已退款，两种场景：售后退款；库存不够退款
)

/*
OrderConsumer Kafka消费者：订单状态变更消息

职责（单一职责原则）：
1. MQ连接管理：创建ConsumerGroup、订阅Topic
2. 消息消费：接收消息、反序列化
3. ACK机制：成功返回nil（自动ACK），失败返回error（触发重试）
4. 异常处理：自动重连、错误日志

不负责：
- 业务逻辑（委托给UseCase层）
- 数据校验（由UseCase层负责）
- 数据库操作（由Repository层负责）
*/
type OrderConsumer struct {
	client         sarama.Client
	producer       sarama.SyncProducer          // 用于发送死信
	commitStockUC  *usecase.CommitStockUseCase  // 支付成功确认
	releaseStockUC *usecase.ReleaseStockUseCase // 取消释放
	refundStockUC  *usecase.RefundStockUseCase  // 退款恢复
	orderCli       orderservice.Client          // 订单服务客户端，用于同步回调更新状态
	logger         logger.LoggerV1
	consumerGrp    sarama.ConsumerGroup // 持有ConsumerGroup用于优雅关闭
}

func NewOrderConsumer(
	client sarama.Client,
	producer sarama.SyncProducer,
	commitUC *usecase.CommitStockUseCase,
	releaseUC *usecase.ReleaseStockUseCase,
	refundUC *usecase.RefundStockUseCase,
	orderCli orderservice.Client,
	l logger.LoggerV1,
) *OrderConsumer {
	return &OrderConsumer{
		client:         client,
		producer:       producer,
		commitStockUC:  commitUC,
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
				c.logger.Error("订单状态变更消费者退出", logger.Error(err))
			}
		}
	}()

	c.logger.Info("OrderConsumer已启动",
		logger.String("topic", TopicOrderStatusUpdate),
		logger.String("consumerGroup", "inventory-consumer"))

	return nil
}

// Consume 消息路由：根据状态分发到不同处理方法，统一ACK不阻塞
func (c *OrderConsumer) Consume(msg *sarama.ConsumerMessage, evt OrderStatusUpdateEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.Info("收到订单状态变更消息",
		logger.Int64("orderID", evt.OrderID),
		logger.Int("status", int(evt.Status)),
		logger.Int("itemCount", len(evt.Items)))

	switch evt.Status {
	case OrderStatusPaid:
		c.handlePaid(ctx, evt)
	case OrderStatusCanceled:
		c.handleCanceled(ctx, evt)
	case OrderStatusRefunded:
		c.handleRefunded(ctx, evt)
	}
	// 上述都是重试（尽力消费成功）+ 死信队列（兜底）
	return nil // 统一ACK，不阻塞
}

// 支付成功 → CommitStock → 更新订单状态为待发货
func (c *OrderConsumer) handlePaid(ctx context.Context, evt OrderStatusUpdateEvent) {
	if len(evt.Items) == 0 {
		c.logger.Error("CommitStock失败：Items为空", logger.Int64("orderID", evt.OrderID))
		return
	}

	changes := make([]domain.StockChange, len(evt.Items))
	for i, item := range evt.Items {
		changes[i] = domain.StockChange{
			ProductID: item.ProductID,
			Quantity:  int32(-item.Quantity),
		}
	}

	cmd := usecase.CommitStockCommand{
		OperationID: buildOrderOperationID(evt.OrderID, "commit"),
		Changes:     changes,
	}

	err := c.executeWithRetry(ctx, "CommitStock", evt, func() error {
		return c.commitStockUC.Execute(ctx, cmd)
	})

	if err != nil {
		// 幂等命中，防止重复消费，之前已处理成功，直接返回
		if errors.Is(err, domain.ErrDuplicateOperation) {
			return
		}

		if errors.Is(err, domain.ErrInsufficientStock) {
			// 库存不足 → 首先尝试释放预扣 + 退款
			c.logger.Warn("CommitStock库存不足，触发退款", logger.Int64("orderID", evt.OrderID))
			releaseCmd := usecase.ReleaseStockCommand{
				OperationID: buildOrderOperationID(evt.OrderID, "release"),
			}
			// Redis的预库存，不像DB库存是真实数据源，没那么重要，失败不重试，有定时任务兜底
			if releaseErr := c.releaseStockUC.Execute(ctx, releaseCmd); releaseErr != nil {
				c.logger.Warn("释放预扣失败（定时任务将兜底修复）", logger.Error(releaseErr))
			}
			// 不管你预库存恢复成功没，转为退款状态
			c.asyncUpdateOrderStatus(ctx, evt, OrderStatusRefunded)
			return
		}
		// 其他错误已在executeWithRetry中处理（重试+死信）
		return
	}
	// 库存扣减成功，转为待发货状态
	c.logger.Info("支付成功，已确认库存", logger.Int64("orderID", evt.OrderID))
	c.asyncUpdateOrderStatus(ctx, evt, OrderStatusToShip)
}

// handleCanceled 订单取消 → ReleaseStock（释放Redis预扣）
// 不需要重试和死信队列，因为：1. 只操作Redis 2. 定时任务会兜底修复缓存不一致
func (c *OrderConsumer) handleCanceled(ctx context.Context, evt OrderStatusUpdateEvent) {
	cmd := usecase.ReleaseStockCommand{
		OperationID: buildOrderOperationID(evt.OrderID, "release"),
	}

	err := c.releaseStockUC.Execute(ctx, cmd)
	if err != nil {
		c.logger.Warn("释放预扣库存失败（定时任务将兜底修复）",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(err))
		return
	}
	c.logger.Info("订单取消，已释放预扣库存", logger.Int64("orderID", evt.OrderID))
}

// 订单退款，两种场景：
// 1. 售后退款（恢复DB+redis）：有commit记录 → 恢复DB库存+Redis预库存
// 2. 库存不足退款(只需恢复redis)：因为DB没扣过，无commit记录 → RefundStock会返回nil（跳过），而Redis预扣已在handlePaid中恢复
func (c *OrderConsumer) handleRefunded(ctx context.Context, evt OrderStatusUpdateEvent) {
	cmd := usecase.RefundStockCommand{
		OperationID: buildOrderOperationID(evt.OrderID, "refund"),
	}

	err := c.executeWithRetry(ctx, "RefundStock", evt, func() error {
		return c.refundStockUC.Execute(ctx, cmd)
	})

	if err == nil {
		c.logger.Info("订单退款，已恢复库存", logger.Int64("orderID", evt.OrderID))
	}
}

// 格式：order_{orderID}_{action}
func buildOrderOperationID(orderID int64, action string) string {
	return fmt.Sprintf("order_%d_%s", orderID, action)
}

// DB库存操作通用重试逻辑，失败进死信队列
// 返回error让调用方判断是否需要特殊处理（如库存不足）
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

		// 幂等命中，返回error让调用方知道操作已执行过
		if errors.Is(lastErr, domain.ErrDuplicateOperation) {
			c.logger.Info(opName+"幂等命中", logger.Int64("orderID", evt.OrderID))
			return lastErr
		}

		// 库存不足，永久性错误，不重试，返回让调用方处理
		if errors.Is(lastErr, domain.ErrInsufficientStock) {
			return lastErr
		}

		c.logger.Warn(opName+"重试",
			logger.Int64("orderID", evt.OrderID),
			logger.Int("attempt", attempt+1),
			logger.Error(lastErr))
	}

	// 重试耗尽，进死信队列
	c.logger.Error(opName+"重试耗尽，进入死信队列",
		logger.Int64("orderID", evt.OrderID),
		logger.Error(lastErr))
	c.sendToStockDeadLetter(evt, opName, lastErr)
	return lastErr
}

// 异步更新订单状态：本地重试 + 死信队列
func (c *OrderConsumer) asyncUpdateOrderStatus(ctx context.Context, evt OrderStatusUpdateEvent, status OrderStatus) {
	go func() {
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
			}

			_, lastErr = c.orderCli.ChangeOrderStatus(ctx, &orderv1.ChangeOrderStatusReq{
				OrderId:     evt.OrderID,
				OrderStatus: uint32(status),
			})
			if lastErr == nil {
				c.logger.Info("更新订单状态成功",
					logger.Int64("orderID", evt.OrderID),
					logger.Int("status", int(status)))
				return
			}

			c.logger.Warn("更新订单状态重试",
				logger.Int64("orderID", evt.OrderID),
				logger.Int("attempt", attempt+1),
				logger.Error(lastErr))
		}

		// 重试耗尽，进死信队列，兜底，保证最终一致性
		c.logger.Error("更新订单状态重试耗尽，进入死信队列",
			logger.Int64("orderID", evt.OrderID),
			logger.Error(lastErr))
		c.sendToOrderDeadLetter(evt, status, lastErr)
	}()
}

// 库存操作失败的死信
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
		c.logger.Error("发送库存死信失败", logger.Int64("orderID", evt.OrderID), logger.Error(err))
	}
}

// 更新订单状态失败的死信
func (c *OrderConsumer) sendToOrderDeadLetter(evt OrderStatusUpdateEvent, targetStatus OrderStatus, lastErr error) {
	msg := map[string]interface{}{
		"event":        evt,
		"targetStatus": targetStatus,
		"error":        lastErr.Error(),
		"time":         time.Now(),
	}
	msgBytes, _ := json.Marshal(msg)
	_, _, err := c.producer.SendMessage(&sarama.ProducerMessage{
		Topic: TopicDeadLetterOrder,
		Key:   sarama.StringEncoder(fmt.Sprintf("%d", evt.OrderID)),
		Value: sarama.ByteEncoder(msgBytes),
	})
	if err != nil {
		c.logger.Error("发送订单死信失败", logger.Int64("orderID", evt.OrderID), logger.Error(err))
	}
}

// 优雅关闭
func (c *OrderConsumer) Stop() error {
	if c.consumerGrp != nil {
		return c.consumerGrp.Close()
	}
	return nil
}
