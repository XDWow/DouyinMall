package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// CheckExpiredJob 定期扫描状态为 pending 的 Order，发现过期了就批量取消
// 不仅仅是操作数据库，还有其他业务逻辑（发送消息、写outbox），所以调用 usecase
// 正常情况下过期订单不多，一次性全部处理；只有异常情况（如系统宕机恢复）才会分批
type CheckExpiredJob struct {
	orderRepo     domain.OrderRepository
	batchCancelUC *usecase.BatchCancelOrderUseCase
	l             logger.LoggerV1
	maxBatchSize  int // 保护性分批：单次最多处理多少订单，防止异常情况
}

func NewCheckExpiredJob(
	orderRepo domain.OrderRepository,
	batchCancelUC *usecase.BatchCancelOrderUseCase,
	l logger.LoggerV1,
) *CheckExpiredJob {
	return &CheckExpiredJob{
		orderRepo:     orderRepo,
		batchCancelUC: batchCancelUC,
		l:             l,
		maxBatchSize:  10000,
	}
}

func (j *CheckExpiredJob) Name() string {
	return "CheckExpiredJob"
}

func (j *CheckExpiredJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	expiredOrders, err := j.orderRepo.FindExpiredOrders(ctx, 0)
	if err != nil {
		j.l.Error("查找过期订单失败", logger.Error(err))
		return err
	}
	if len(expiredOrders) == 0 {
		return nil
	}
	j.l.Info("发现过期订单", logger.Int("count", len(expiredOrders)))

	// 分批处理（异常情况）
	if len(expiredOrders) > j.maxBatchSize {
		j.l.Warn("过期订单数量超过预期，将分批处理",
			logger.Int("total", len(expiredOrders)),
			logger.Int("batchSize", j.maxBatchSize))
		return j.processByBatch(ctx, expiredOrders)
	}

	err = j.batchCancelUC.Execute(ctx, expiredOrders)
	if err != nil {
		j.l.Error("批量取消过期订单失败",
			logger.Error(err),
			logger.Int("orderCount", len(expiredOrders)))
		return err
	}
	j.l.Info("批量取消过期订单成功", logger.Int("count", len(expiredOrders)))
	return nil
}

func (j *CheckExpiredJob) processByBatch(ctx context.Context, orders []*domain.Order) error {
	for i := 0; i < len(orders); i += j.maxBatchSize {
		end := i + j.maxBatchSize
		if end > len(orders) {
			end = len(orders)
		}
		batch := orders[i:end]

		err := j.batchCancelUC.Execute(ctx, batch)
		if err != nil {
			j.l.Error("分批取消过期订单失败",
				logger.Error(err),
				logger.Int("batchIndex", i/j.maxBatchSize+1),
				logger.Int("batchSize", len(batch)))
			return err
		}
		j.l.Info("完成一批订单取消",
			logger.Int("batchIndex", i/j.maxBatchSize+1),
			logger.Int("batchSize", len(batch)))
	}
	return nil
}
