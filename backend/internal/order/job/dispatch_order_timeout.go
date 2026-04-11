package job

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	paymentservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
)

type DispatchOrderTimeoutJob struct {
	delayQueue    domain.DelayQueue
	paymentCli    paymentservice.Client
	orderRepo     domain.OrderRepository
	outboxRepo    domain.OutboxRepository
	producer      mq.SaramaProducer
	tx            domain.TxManager
	batchCancelUC BatchCancelOrderExecutor
	l             logger.LoggerV1
}

type BatchCancelOrderExecutor interface {
	Execute(ctx context.Context, orderIDs []int64) error
}

func NewDispatchOrderTimeoutJob(
	delayQueue domain.DelayQueue,
	paymentCli paymentservice.Client,
	orderRepo domain.OrderRepository,
	outboxRepo domain.OutboxRepository,
	producer mq.SaramaProducer,
	tx domain.TxManager,
	batchCancelUC BatchCancelOrderExecutor,
	l logger.LoggerV1,
) *DispatchOrderTimeoutJob {
	return &DispatchOrderTimeoutJob{
		delayQueue:    delayQueue,
		paymentCli:    paymentCli,
		orderRepo:     orderRepo,
		outboxRepo:    outboxRepo,
		producer:      producer,
		tx:            tx,
		batchCancelUC: batchCancelUC,
		l:             l,
	}
}

func (j *DispatchOrderTimeoutJob) Name() string {
	return "DispatchOrderTimeoutJob"
}

// Run 从延时队列取出到期订单 ID，逐笔判断是否应取消：
// - 本地状态非 CREATED：跳过；
// - CREATED：向支付确认；若已支付则同步为 PAID 并写 outbox 发状态事件；
// - 未支付：进入批量取消。
func (j *DispatchOrderTimeoutJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orderIDs, err := j.delayQueue.DrainDue(ctx, time.Now())
	if err != nil {
		j.l.Error("拉取到期超时订单失败", logger.Error(err))
		return err
	}
	if len(orderIDs) == 0 {
		return nil
	}

	toCancelIDs := make([]int64, 0, len(orderIDs))
	for i, orderID := range orderIDs {
		shouldCancel, evalErr := j.shouldCancelDueOrder(ctx, orderID)
		if evalErr != nil {
			j.l.Warn("判断超时订单是否可取消失败",
				logger.Int64("orderID", orderID),
				logger.Error(evalErr))
			j.requeue(ctx, append(append([]int64(nil), toCancelIDs...), orderIDs[i:]...))
			return evalErr
		}
		if !shouldCancel {
			continue
		}

		toCancelIDs = append(toCancelIDs, orderID)
	}

	if len(toCancelIDs) == 0 {
		return nil
	}

	if err := j.batchCancelUC.Execute(ctx, toCancelIDs); err != nil {
		j.l.Error("取消超时订单失败",
			logger.Error(err),
			logger.Int("count", len(toCancelIDs)))
		j.requeue(ctx, toCancelIDs)
		return err
	}
	return nil
}

// shouldCancelDueOrder 仅当本地仍为 CREATED 时才继续向支付侧确认。
func (j *DispatchOrderTimeoutJob) shouldCancelDueOrder(ctx context.Context, orderID int64) (bool, error) {
	order, err := j.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if order.Status != domain.OrderStatusCreated {
		return false, nil
	}

	status, err := j.confirmPayment(ctx, orderID)
	if err != nil {
		return false, err
	}
	if status == paymentv1.PaymentStatus_PaymentStatusSuccess {
		if err := j.syncPaidOrder(ctx, &order); err != nil {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

// syncPaidOrder 支付已成功：把本地订单更新为 PAID 并追加 outbox 状态事件。
func (j *DispatchOrderTimeoutJob) syncPaidOrder(ctx context.Context, order *domain.Order) error {
	paidOrder := *order
	paidOrder.Status = domain.OrderStatusPaid
	event := domain.BuildOrderStatusUpdateEvent(&paidOrder)

	var outboxID int64
	err := j.tx.Tx(ctx, func(ctx context.Context) error {
		if err := j.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusCreated, domain.OrderStatusPaid); err != nil {
			if errors.Is(err, domain.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var addErr error
		outboxID, addErr = j.outboxRepo.Add(ctx, domain.EventTypeOrderStatusChanged, event)
		return addErr
	})
	if err != nil {
		return err
	}
	if outboxID == 0 {
		return nil
	}

	go j.sendStatusEvent(outboxID, event)
	return nil
}

func (j *DispatchOrderTimeoutJob) sendStatusEvent(outboxID int64, event domain.OrderStatusUpdateEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := j.producer.SendMessage(ctx, event); err != nil {
		j.l.Error("发送订单状态变更事件失败",
			logger.Error(err),
			logger.Int64("orderID", event.OrderID),
			logger.Int64("outboxID", outboxID))
		return
	}

	if err := j.outboxRepo.MarkSent(ctx, outboxID); err != nil {
		j.l.Error("标记 outbox 已发送失败",
			logger.Error(err),
			logger.Int64("orderID", event.OrderID),
			logger.Int64("outboxID", outboxID))
	}
}

func (j *DispatchOrderTimeoutJob) confirmPayment(ctx context.Context, orderID int64) (paymentv1.PaymentStatus, error) {
	confirmCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := j.paymentCli.ConfirmPayment(confirmCtx, &paymentv1.ConfirmPaymentRequest{
		BizTradeNo: strconv.FormatInt(orderID, 10),
	})
	if err != nil {
		return paymentv1.PaymentStatus_PaymentStatusUnknown, err
	}
	return resp.GetStatus(), nil
}

func (j *DispatchOrderTimeoutJob) requeue(ctx context.Context, orderIDs []int64) {
	retryAt := time.Now()
	for _, orderID := range orderIDs {
		if err := j.delayQueue.Enqueue(ctx, orderID, retryAt); err != nil {
			j.l.Error("超时订单重新入队失败",
				logger.Int64("orderID", orderID),
				logger.Error(err))
		}
	}
}


