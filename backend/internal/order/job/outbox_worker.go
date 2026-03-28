package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// OutboxWorkerJob 定期扫描 pending 的 outboxEvent，进行发送
// 这是慢路径的兜底机制，确保即使快路径失败，消息最终也能被发送（最终数据一致性）
// Outbox 就是记录待发送消息、重试次数和下次重试时间，再由定时任务持续扫描，把还能重试的消息继续发出去。
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

// 简单点，先不考虑分布式定时任务
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
		// 生产者批量发送
		j.processBatch(ctx, outboxEvents)

		// 如果本批次数量小于batchSize，说明已经是最后一批
		if len(outboxEvents) < j.batchSize {
			break
		}

		offset += j.batchSize
	}

	return nil
}

// 性能优化在发送层：使用批量发送API
// 失败隔离：每个消息独立，精确处理失败
func (j *OutboxWorkerJob) processBatch(ctx context.Context, outboxEvents []domain.OutboxEvent) {
	events := make([]domain.OrderStatusUpdateEvent, 0, len(outboxEvents))
	for _, outboxEvent := range outboxEvents {
		events = append(events, outboxEvent.Event)
	}

	errs := j.producer.SendMessages(ctx, events)

	successIDs := make([]int64, 0, len(outboxEvents))
	failedIDs := make([]int64, 0)

	// 处理结果：失败被隔离而不是放大
	if errs == nil {
		// 全部成功
		for _, outboxEvent := range outboxEvents {
			successIDs = append(successIDs, outboxEvent.ID)
		}
	} else {
		// 逐个检查结果
		for i, err := range errs {
			if err != nil {
				j.l.Error("发送outbox事件失败",
					logger.Error(err),
					logger.Int64("outboxID", outboxEvents[i].ID),
					logger.Int64("orderID", outboxEvents[i].Event.OrderID))

				failedIDs = append(failedIDs, outboxEvents[i].ID)

				// 增加重试发送的次数，这里有分支判断（标记失败+DLQ+告警），所以不用批量
				retry, err := j.outboxRepo.IncreaseRetry(ctx, outboxEvents[i].ID)
				if err != nil {
					j.l.Error("增加outbox重试次数失败",
						logger.Error(err),
						logger.Int64("outboxID", outboxEvents[i].ID))
				} else if retry > j.maxRetry {
					j.l.Warn("outbox事件重试次数达到上限，需人工介入处理",
						logger.Int64("outboxID", outboxEvents[i].ID),
						logger.Int64("orderID", outboxEvents[i].Event.OrderID),
						logger.Int("maxRetry", j.maxRetry))
					err = j.outboxRepo.MarkFailed(ctx, outboxEvents[i].ID)
					if err != nil {
						j.l.Error("标记outbox事件为失败状态失败",
							logger.Error(err),
							logger.Int64("outboxID", outboxEvents[i].ID))
					}
					// 后续入死信队列+告警
				}

			} else {
				successIDs = append(successIDs, outboxEvents[i].ID)
			}
		}
	}

	// 批量标记成功发送的事件
	if len(successIDs) > 0 {
		if err := j.outboxRepo.BatchMarkSent(ctx, successIDs); err != nil {
			j.l.Error("批量标记outbox为已发送失败",
				logger.Error(err),
				logger.Int("successCount", len(successIDs)))
		} else {
			j.l.Info("批量发送outbox事件成功",
				logger.Int("successCount", len(successIDs)))
		}
	}

	if len(failedIDs) > 0 {
		j.l.Warn("部分outbox事件发送失败，将在下次重试",
			logger.Int("failedCount", len(failedIDs)))
	}
}
