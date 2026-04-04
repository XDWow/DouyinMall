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

// 鏈湴涓嶆槸 CREATED锛岀洿鎺ヨ烦杩囥€?
// 鏈湴鏄?CREATED锛屽啀鍘绘敮浠樼‘璁ゃ€?
// 宸叉敮浠樺氨鐩存帴鏉′欢鏇存柊鎴?PAID锛屽苟鍐?outbox / 鍙戠姸鎬佷簨浠躲€?
// 鏈敮浠樻墠杩涘叆鎵归噺鍙栨秷銆?
func (j *DispatchOrderTimeoutJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orderIDs, err := j.delayQueue.DrainDue(ctx, time.Now())
	if err != nil {
		j.l.Error("鎷夊彇鍒版湡瓒呮椂璁㈠崟澶辫触", logger.Error(err))
		return err
	}
	if len(orderIDs) == 0 {
		return nil
	}

	toCancelIDs := make([]int64, 0, len(orderIDs))
	for i, orderID := range orderIDs {
		shouldCancel, evalErr := j.shouldCancelDueOrder(ctx, orderID)
		if evalErr != nil {
			j.l.Warn("鍒ゆ柇瓒呮椂璁㈠崟鏄惁鍙彇娑堝け璐?,
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
		j.l.Error("鍙栨秷瓒呮椂璁㈠崟澶辫触",
			logger.Error(err),
			logger.Int("count", len(toCancelIDs)))
		j.requeue(ctx, toCancelIDs)
		return err
	}
	return nil
}

// 鍒ゆ柇璁㈠崟鏄惁搴旇璧拌秴鏃跺彇娑堬細鍙湁鏈湴鐘舵€佷粛涓?CREATED 鎵嶇户缁‘璁ゆ敮浠樸€?
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

// 杩欓噷琛ㄧず璁㈠崟宸叉敮浠橈紝闇€瑕佹妸鏈湴璁㈠崟鐘舵€佸悓姝ユ垚 PAID 骞跺彂閫佺姸鎬佸彉鏇翠簨浠躲€?
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
		j.l.Error("鍙戦€佽鍗曠姸鎬佸彉鏇翠簨浠跺け璐?,
			logger.Error(err),
			logger.Int64("orderID", event.OrderID),
			logger.Int64("outboxID", outboxID))
		return
	}

	if err := j.outboxRepo.MarkSent(ctx, outboxID); err != nil {
		j.l.Error("鏍囪 outbox 宸插彂閫佸け璐?,
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
			j.l.Error("瓒呮椂璁㈠崟閲嶆柊鍏ラ槦澶辫触",
				logger.Int64("orderID", orderID),
				logger.Error(err))
		}
	}
}


