package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type PaymentOutboxWorkerJob struct {
	outboxRepo domain.PaymentOutboxRepository
	producer   mq.PaymentStatusProducer
	l          logger.LoggerV1
	batchSize  int
	maxRetry   int
}

func NewPaymentOutboxWorkerJob(
	outboxRepo domain.PaymentOutboxRepository,
	producer mq.PaymentStatusProducer,
	l logger.LoggerV1,
) *PaymentOutboxWorkerJob {
	return &PaymentOutboxWorkerJob{
		outboxRepo: outboxRepo,
		producer:   producer,
		l:          l,
		batchSize:  100,
		maxRetry:   5,
	}
}

func (j *PaymentOutboxWorkerJob) Name() string {
	return "payment_outbox_worker_job"
}

func (j *PaymentOutboxWorkerJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	offset := 0
	for {
		events, err := j.outboxRepo.ListPending(ctx, offset, j.batchSize)
		if err != nil {
			j.l.Error("查询待投递支付 outbox 失败", logger.Error(err))
			return err
		}
		if len(events) == 0 {
			return nil
		}

		j.processBatch(ctx, events)
		if len(events) < j.batchSize {
			return nil
		}
		offset += j.batchSize
	}
}

func (j *PaymentOutboxWorkerJob) processBatch(ctx context.Context, outboxEvents []domain.PaymentOutboxEvent) {
	events := make([]domain.PaymentStatusUpdateEvent, 0, len(outboxEvents))
	for _, outboxEvent := range outboxEvents {
		events = append(events, outboxEvent.Event)
	}

	errs := j.producer.SendMessages(ctx, events)
	successIDs := make([]int64, 0, len(outboxEvents))

	if errs == nil {
		for _, outboxEvent := range outboxEvents {
			successIDs = append(successIDs, outboxEvent.ID)
		}
	} else {
		for i, err := range errs {
			if err == nil {
				successIDs = append(successIDs, outboxEvents[i].ID)
				continue
			}
			j.l.Error("投递支付 outbox 事件失败",
				logger.Error(err),
				logger.Int64("outboxID", outboxEvents[i].ID),
				logger.Int64("orderID", outboxEvents[i].Event.OrderID))
			retry, retryErr := j.outboxRepo.IncreaseRetry(ctx, outboxEvents[i].ID)
			if retryErr != nil {
				j.l.Error("增加支付 outbox 重试次数失败",
					logger.Error(retryErr),
					logger.Int64("outboxID", outboxEvents[i].ID))
			} else if retry > j.maxRetry {
				j.l.Warn("支付 outbox 重试次数耗尽",
					logger.Int64("outboxID", outboxEvents[i].ID),
					logger.Int64("orderID", outboxEvents[i].Event.OrderID),
					logger.Int("maxRetry", j.maxRetry))
				if markErr := j.outboxRepo.MarkFailed(ctx, outboxEvents[i].ID); markErr != nil {
					j.l.Error("标记支付 outbox 为失败状态失败",
						logger.Error(markErr),
						logger.Int64("outboxID", outboxEvents[i].ID))
				}
			}
		}
	}

	if len(successIDs) > 0 {
		if err := j.outboxRepo.BatchMarkSent(ctx, successIDs); err != nil {
			j.l.Error("标记支付 outbox 已投递失败",
				logger.Error(err),
				logger.Int("count", len(successIDs)))
		}
	}
}
