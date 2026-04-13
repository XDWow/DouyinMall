package usecase

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type BatchCancelOrderUseCase struct {
	orderRepo  domain.OrderRepository
	outboxRepo domain.OutboxRepository
	producer   mq.SaramaProducer
	tx         domain.TxManager
	log        logger.LoggerV1
}

func NewBatchCancelOrderUseCase(
	orderRepo domain.OrderRepository,
	outboxRepo domain.OutboxRepository,
	producer mq.SaramaProducer,
	tx domain.TxManager,
	log logger.LoggerV1,
) *BatchCancelOrderUseCase {
	return &BatchCancelOrderUseCase{
		orderRepo:  orderRepo,
		outboxRepo: outboxRepo,
		producer:   producer,
		tx:         tx,
		log:        log,
	}
}

func (uc *BatchCancelOrderUseCase) Execute(ctx context.Context, orderIDs []int64) error {
	if len(orderIDs) == 0 {
		return nil
	}

	var outboxIDs []int64
	var events []domain.OrderStatusUpdateEvent
	var cancelIDs []int64

	if err := uc.tx.Tx(ctx, func(ctx context.Context) error {
		// 超时取消是后台兜底任务，不抢支付成功链路的锁。
		// 先普通读出候选订单，再以条件更新是否成功作为“真的取消成功”的判定依据。
		orders, err := uc.orderRepo.FindByIDs(ctx, orderIDs)
		if err != nil {
			return err
		}

		toCancelIDs := make([]int64, 0, len(orders))
		events = make([]domain.OrderStatusUpdateEvent, 0, len(orders))
		payloads := make([]any, 0, len(orders))

		for _, order := range orders {
			if order.Status != domain.OrderStatusCreated {
				continue
			}
			toCancelIDs = append(toCancelIDs, order.ID)

			canceledOrder := *order
			canceledOrder.Status = domain.OrderStatusCanceled
			event := domain.BuildOrderStatusUpdateEvent(&canceledOrder)
			events = append(events, event)
			payloads = append(payloads, event)
		}

		if len(toCancelIDs) == 0 {
			return nil
		}

		if err := uc.orderRepo.BatchUpdateStatus(ctx, toCancelIDs, domain.OrderStatusCreated, domain.OrderStatusCanceled); err != nil {
			return err
		}

		cancelIDs = toCancelIDs

		var addErr error
		outboxIDs, addErr = uc.outboxRepo.BatchAdd(ctx, domain.EventTypeOrderStatusChanged, payloads)
		return addErr
	}); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	go uc.batchSendMessages(cancelIDs, outboxIDs, events)
	return nil
}

func (uc *BatchCancelOrderUseCase) batchSendMessages(orderIDs, outboxIDs []int64, events []domain.OrderStatusUpdateEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errs := uc.producer.SendMessages(ctx, events)

	successOutboxIDs := make([]int64, 0, len(outboxIDs))
	failedOrderIDs := make([]int64, 0)

	if errs == nil {
		successOutboxIDs = append(successOutboxIDs, outboxIDs...)
	} else {
		for i, err := range errs {
			if err == nil {
				successOutboxIDs = append(successOutboxIDs, outboxIDs[i])
				continue
			}

			failedOrderIDs = append(failedOrderIDs, orderIDs[i])
			uc.log.Error("发送取消订单事件失败", logger.Error(err), logger.Int64("orderID", orderIDs[i]), logger.Int64("outboxID", outboxIDs[i]))
		}
	}

	if len(successOutboxIDs) > 0 {
		if err := uc.outboxRepo.BatchMarkSent(ctx, successOutboxIDs); err != nil {
			uc.log.Error("批量标记 outbox 已发送失败", logger.Error(err), logger.Int("count", len(successOutboxIDs)))
		}
	}
	if len(failedOrderIDs) > 0 {
		uc.log.Warn("部分取消订单事件发送失败", logger.Int("count", len(failedOrderIDs)))
	}
}
