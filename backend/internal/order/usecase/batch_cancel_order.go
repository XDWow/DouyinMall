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
		// 蹇呴』瑕佹煡鍒颁俊鎭紝鍚庣画鍙?event 瑕佺敤锛屼笖浜嬪姟涓€寮€濮嬪氨褰撳墠璇伙紝鎶婅繖浜涢兘閿佷綇锛岄伩鍏嶅苟鍙戦棶棰?
		orders, err := uc.orderRepo.FindByIDsForUpdate(ctx, orderIDs)
		if err != nil {
			return err
		}

		events = make([]domain.OrderStatusUpdateEvent, 0, len(orders))
		payloads := make([]any, 0, len(orders))
		cancelIDs = make([]int64, 0, len(orders))

		for _, order := range orders {
			if order.Status != domain.OrderStatusCreated {
				continue
			}

			canceledOrder := *order
			canceledOrder.Status = domain.OrderStatusCanceled
			event := domain.BuildOrderStatusUpdateEvent(&canceledOrder)

			cancelIDs = append(cancelIDs, order.ID)
			events = append(events, event)
			payloads = append(payloads, event)
		}

		if len(events) == 0 {
			return nil
		}

		if err := uc.orderRepo.BatchUpdateStatus(ctx, cancelIDs, domain.OrderStatusCreated, domain.OrderStatusCanceled); err != nil {
			return err
		}
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
			uc.log.Error("鍙戦€佸彇娑堣鍗曚簨浠跺け璐?, logger.Error(err), logger.Int64("orderID", orderIDs[i]), logger.Int64("outboxID", outboxIDs[i]))
		}
	}

	if len(successOutboxIDs) > 0 {
		if err := uc.outboxRepo.BatchMarkSent(ctx, successOutboxIDs); err != nil {
			uc.log.Error("鎵归噺鏍囪 outbox 宸插彂閫佸け璐?, logger.Error(err), logger.Int("count", len(successOutboxIDs)))
		}
	}
	if len(failedOrderIDs) > 0 {
		uc.log.Warn("閮ㄥ垎鍙栨秷璁㈠崟浜嬩欢鍙戦€佸け璐?, logger.Int("count", len(failedOrderIDs)))
	}
}


