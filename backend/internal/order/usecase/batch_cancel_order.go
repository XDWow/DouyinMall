package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// 用于定时任务扫描超时订单并批量取消
type BatchCancelOrderUseCase struct {
	orderRepo  domain.OrderRepository
	outboxRepo domain.OutboxRepository
	producer   mq.SaramaProducer
	tx         TxManager
	log        logger.LoggerV1
	maxRetry   int
}

func NewBatchCancelOrderUseCase(
	orderRepo domain.OrderRepository,
	outboxRepo domain.OutboxRepository,
	producer mq.SaramaProducer,
	tx TxManager,
	log logger.LoggerV1,
) *BatchCancelOrderUseCase {
	return &BatchCancelOrderUseCase{
		orderRepo:  orderRepo,
		outboxRepo: outboxRepo,
		producer:   producer,
		tx:         tx,
		log:        log,
		maxRetry:   5, // 最大重试5次
	}
}

// Execute 执行批量取消订单
// 1. 批量更新订单状态
// 2. 批量写入outbox事件（慢路径兜底）
// 3. 批量发送MQ消息（快路径）
func (uc *BatchCancelOrderUseCase) Execute(ctx context.Context, orders []*domain.Order) error {
	if len(orders) == 0 {
		return nil
	}

	orderIDs := make([]int64, 0, len(orders))
	events := make([]any, 0, len(orders))

	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
		events = append(events, domain.OrderStatusUpdateEvent{
			OrderID: order.ID,
			Status:  domain.OrderStatusCanceled,
		})
	}

	err := uc.tx.WithTx(ctx, func(ctx context.Context) error {
		err := uc.orderRepo.BatchUpdateStatus(ctx, orderIDs, domain.OrderStatusPending, domain.OrderStatusCanceled)
		if err != nil {
			uc.log.Error("批量更新订单状态失败",
				logger.Error(err),
				logger.Int("orderCount", len(orderIDs)))
			return err
		}

		// 批量写入outbox（慢路径兜底，保证生产者消息不丢）
		err = uc.outboxRepo.BatchAdd(ctx, OrderStatusChanged, events)
		if err != nil {
			uc.log.Error("批量保存outbox失败",
				logger.Error(err),
				logger.Int("orderCount", len(orderIDs)))
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	uc.log.Info("批量取消订单成功",
		logger.Int("orderCount", len(orderIDs)))

	// 快路径：异步批量发送MQ消息
	go uc.batchSendMessages(orderIDs, events)

	return nil
}

// batchSendMessages 批量发送MQ消息（快路径）
// 性能优化在发送层：使用MQ的批量发送API，而不是业务上的批量
// 失败隔离：每个消息独立处理，防止部分失败放大
func (uc *BatchCancelOrderUseCase) batchSendMessages(orderIDs []int64, events []any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mqEvents := make([]domain.OrderStatusUpdateEvent, 0, len(events))
	for _, event := range events {
		mqEvents = append(mqEvents, event.(domain.OrderStatusUpdateEvent))
	}

	// 批量发送：性能优化在MQ生产者，每个消息仍然独立
	errs := uc.producer.SendMessages(ctx, mqEvents)

	successIDs := make([]int64, 0, len(orderIDs))
	failedIDs := make([]int64, 0)

	// 根据 errs 进行后续处理
	if errs == nil {
		successIDs = orderIDs
	} else {
		for i, err := range errs {
			if err != nil {
				uc.log.Error("订单状态变化事件发送失败",
					logger.Error(err),
					logger.Int64("orderId", mqEvents[i].OrderID))
				failedIDs = append(failedIDs, orderIDs[i])

				// 失败了：增加重试次数，失败处理逻辑包含"分支决策"（DLQ+告警），就不再具备批量条件
				retryCount, e := uc.outboxRepo.IncreaseRetry(ctx, orderIDs[i])
				if e != nil {
					uc.log.Error("增加重试次数失败",
						logger.Error(e),
						logger.Int64("orderId", orderIDs[i]))
				} else if retryCount > uc.maxRetry {
					// 达到最大重试次数，标记为失败（DLQ）
					if err := uc.outboxRepo.MarkFailed(ctx, orderIDs[i]); err != nil {
						uc.log.Error("标记事件失败状态失败",
							logger.Error(err),
							logger.Int64("orderId", orderIDs[i]))
					} else {
						uc.log.Warn("事件达到最大重试次数，已标记为失败",
							logger.Int64("orderId", orderIDs[i]),
							logger.Int("retryCount", retryCount))
						// TODO: 进入DLQ，发送告警通知
					}
				}
			} else {
				successIDs = append(successIDs, orderIDs[i])
			}
		}
	}

	// 批量标记成功发送的消息
	if len(successIDs) > 0 {
		if err := uc.outboxRepo.BatchMarkSent(ctx, successIDs); err != nil {
			uc.log.Error("批量标记outbox为已发送失败",
				logger.Error(err),
				logger.Int("successCount", len(successIDs)))
		} else {
			uc.log.Info("批量发送订单取消事件成功",
				logger.Int("successCount", len(successIDs)))
		}
	}

	if len(failedIDs) > 0 {
		uc.log.Warn("部分订单状态变化事件发送失败",
			logger.Int("failedCount", len(failedIDs)))
	}
}
