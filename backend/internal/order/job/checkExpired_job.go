package job

import (
	"context"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	paymentservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1/paymentservice"
)

// 延迟关单老做法：扫描过期的订单，关闭；问题：扫描间隔不好确定，太频繁成本大，每次扫描整个订单表；太久：无法准时关闭
// 现在延迟关单交给延时队列，本定时任务执行间隔久一点，用来兜底关单
type CheckExpiredJob struct {
	orderRepo     domain.OrderRepository
	paymentCli    paymentservice.Client
	batchCancelUC *usecase.BatchCancelOrderUseCase
	l             logger.LoggerV1
	maxBatchSize  int
}

func NewCheckExpiredJob(
	orderRepo domain.OrderRepository,
	paymentCli paymentservice.Client,
	batchCancelUC *usecase.BatchCancelOrderUseCase,
	l logger.LoggerV1,
) *CheckExpiredJob {
	return &CheckExpiredJob{
		orderRepo:     orderRepo,
		paymentCli:    paymentCli,
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
		j.l.Error("查询过期订单失败", logger.Error(err))
		return err
	}
	if len(expiredOrders) == 0 {
		return nil
	}

	cancelableOrderIDs := make([]int64, 0, len(expiredOrders))
	for _, order := range expiredOrders {
		status, confirmErr := j.confirmPayment(ctx, order)
		if confirmErr != nil {
			j.l.Warn("支付确认失败，跳过取消订单",
				logger.Int64("orderID", order.ID),
				logger.Error(confirmErr))
			continue
		}
		if status == paymentv1.PaymentStatus_PaymentStatusSuccess {
			j.l.Info("订单已支付，跳过取消",
				logger.Int64("orderID", order.ID))
			continue
		}
		cancelableOrderIDs = append(cancelableOrderIDs, order.ID)
	}

	if len(cancelableOrderIDs) == 0 {
		return nil
	}

	if len(cancelableOrderIDs) > j.maxBatchSize {
		j.l.Warn("过期订单数量超过批处理阈值",
			logger.Int("total", len(cancelableOrderIDs)),
			logger.Int("batchSize", j.maxBatchSize))
		return j.processByBatch(ctx, cancelableOrderIDs)
	}

	if err = j.batchCancelUC.Execute(ctx, cancelableOrderIDs); err != nil {
		j.l.Error("批量取消过期订单失败",
			logger.Error(err),
			logger.Int("orderCount", len(cancelableOrderIDs)))
		return err
	}
	j.l.Info("批量取消过期订单成功", logger.Int("count", len(cancelableOrderIDs)))
	return nil
}

func (j *CheckExpiredJob) confirmPayment(ctx context.Context, order *domain.Order) (paymentv1.PaymentStatus, error) {
	confirmCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := j.paymentCli.ConfirmPayment(confirmCtx, &paymentv1.ConfirmPaymentRequest{
		BizTradeNo: strconv.FormatInt(order.ID, 10),
	})
	if err != nil {
		return paymentv1.PaymentStatus_PaymentStatusUnknown, err
	}
	return resp.GetStatus(), nil
}

func (j *CheckExpiredJob) processByBatch(ctx context.Context, orderIDs []int64) error {
	for i := 0; i < len(orderIDs); i += j.maxBatchSize {
		end := i + j.maxBatchSize
		if end > len(orderIDs) {
			end = len(orderIDs)
		}
		batch := orderIDs[i:end]

		if err := j.batchCancelUC.Execute(ctx, batch); err != nil {
			j.l.Error("批量取消过期订单失败",
				logger.Error(err),
				logger.Int("batchIndex", i/j.maxBatchSize+1),
				logger.Int("batchSize", len(batch)))
			return err
		}
		j.l.Info("完成一批过期订单取消",
			logger.Int("batchIndex", i/j.maxBatchSize+1),
			logger.Int("batchSize", len(batch)))
	}
	return nil
}
