package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// OutboxWorkerJob 定时扫描 pending 的 outbox 记录并尝试发送到 Kafka
// 这是慢路径兜底：即使同步发送失败，消息仍有机会最终被发出（最终与 DB 一致）
// Outbox 记录待发送内容、重试次数与下次重试时间；本任务持续扫描，把仍可重试的消息继续投递
type OutboxWorkerJob struct {
	outboxRepo domain.OutboxRepository
	producer   mq.SaramaProducer
	l          logger.LoggerV1
	batchSize  int
	maxRetry   int
}

func NewOutboxWorkerJob(
	outboxRepo domain.OutboxRepository,
	producer mq.SaramaProducer,
	l logger.LoggerV1,
) *OutboxWorkerJob {
	return &OutboxWorkerJob{
		outboxRepo: outboxRepo,
		producer:   producer,
		l:          l,
		batchSize:  100,
		maxRetry:   5,
	}
}

func (j *OutboxWorkerJob) Name() string {
	return "OutboxWorkerJob"
}

func (j *OutboxWorkerJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	offset := 0
	for {
		outboxEvents, err := j.outboxRepo.ListPending(ctx, offset, j.batchSize)
		if err != nil {
			j.l.Error("查询待发送 outbox 事件失败", logger.Error(err))
			return err
		}
		if len(outboxEvents) == 0 {
			break
		}
		j.l.Info("发现待发送 outbox 事件", logger.Int("count", len(outboxEvents)))

		j.processBatch(ctx, outboxEvents)

		if len(outboxEvents) < j.batchSize {
			break
		}

		offset += j.batchSize
	}

	return nil
}

func (j *OutboxWorkerJob) processBatch(ctx context.Context, outboxEvents []domain.OutboxEvent) {
	events := make([]domain.OrderStatusUpdateEvent, 0, len(outboxEvents))
	for _, outboxEvent := range outboxEvents {
		events = append(events, outboxEvent.Event)
	}

	errs := j.producer.SendMessages(ctx, events)

	successIDs := make([]int64, 0, len(outboxEvents))
	failedIDs := make([]int64, 0)

	if errs == nil {
		for _, outboxEvent := range outboxEvents {
			successIDs = append(successIDs, outboxEvent.ID)
		}
	} else {
		for i, err := range errs {
			if err != nil {
				j.l.Error("发送 outbox 事件失败",
					logger.Error(err),
					logger.Int64("outboxID", outboxEvents[i].ID),
					logger.Int64("orderID", outboxEvents[i].Event.OrderID))

				failedIDs = append(failedIDs, outboxEvents[i].ID)

				retry, err := j.outboxRepo.IncreaseRetry(ctx, outboxEvents[i].ID)
				if err != nil {
					j.l.Error("增加 outbox 重试次数失败",
						logger.Error(err),
						logger.Int64("outboxID", outboxEvents[i].ID))
				} else if retry > j.maxRetry {
					j.l.Warn("outbox 事件重试次数已达上限，需人工介入处理",
						logger.Int64("outboxID", outboxEvents[i].ID),
						logger.Int64("orderID", outboxEvents[i].Event.OrderID),
						logger.Int("maxRetry", j.maxRetry))
					err = j.outboxRepo.MarkFailed(ctx, outboxEvents[i].ID)
					if err != nil {
						j.l.Error("标记 outbox 事件为失败状态失败",
							logger.Error(err),
							logger.Int64("outboxID", outboxEvents[i].ID))
					}
					// 后续可接死信队列与告警
				}

			} else {
				successIDs = append(successIDs, outboxEvents[i].ID)
			}
		}
	}

	// 批量标记已成功发送的事件
	if len(successIDs) > 0 {
		if err := j.outboxRepo.BatchMarkSent(ctx, successIDs); err != nil {
			j.l.Error("批量标记 outbox 为已发送失败",
				logger.Error(err),
				logger.Int("successCount", len(successIDs)))
		} else {
			j.l.Info("批量发送 outbox 事件成功",
				logger.Int("successCount", len(successIDs)))
		}
	}

	if len(failedIDs) > 0 {
		j.l.Warn("部分 outbox 事件发送失败，将在下次重试",
			logger.Int("failedCount", len(failedIDs)))
	}
}
