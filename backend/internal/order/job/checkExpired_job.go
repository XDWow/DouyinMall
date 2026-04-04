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

// 寤惰繜鍏冲崟鑰佸仛娉曪細鎵弿杩囨湡鐨勮鍗曪紝鍏抽棴锛涢棶棰橈細鎵弿闂撮殧涓嶅ソ纭畾锛屽お棰戠箒鎴愭湰澶э紝姣忔鎵弿鏁翠釜璁㈠崟琛紱澶箙锛氭棤娉曞噯鏃跺叧闂?
// 鐜板湪寤惰繜鍏冲崟浜ょ粰寤舵椂闃熷垪锛屾湰瀹氭椂浠诲姟鎵ц闂撮殧涔呬竴鐐癸紝鐢ㄦ潵鍏滃簳鍏冲崟
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
		j.l.Error("鏌ヨ杩囨湡璁㈠崟澶辫触", logger.Error(err))
		return err
	}
	if len(expiredOrders) == 0 {
		return nil
	}

	cancelableOrderIDs := make([]int64, 0, len(expiredOrders))
	for _, order := range expiredOrders {
		status, confirmErr := j.confirmPayment(ctx, order)
		if confirmErr != nil {
			j.l.Warn("鏀粯纭澶辫触锛岃烦杩囧彇娑堣鍗?,
				logger.Int64("orderID", order.ID),
				logger.Error(confirmErr))
			continue
		}
		if status == paymentv1.PaymentStatus_PaymentStatusSuccess {
			j.l.Info("璁㈠崟宸叉敮浠橈紝璺宠繃鍙栨秷",
				logger.Int64("orderID", order.ID))
			continue
		}
		cancelableOrderIDs = append(cancelableOrderIDs, order.ID)
	}

	if len(cancelableOrderIDs) == 0 {
		return nil
	}

	if len(cancelableOrderIDs) > j.maxBatchSize {
		j.l.Warn("杩囨湡璁㈠崟鏁伴噺瓒呰繃鎵瑰鐞嗛槇鍊?,
			logger.Int("total", len(cancelableOrderIDs)),
			logger.Int("batchSize", j.maxBatchSize))
		return j.processByBatch(ctx, cancelableOrderIDs)
	}

	if err = j.batchCancelUC.Execute(ctx, cancelableOrderIDs); err != nil {
		j.l.Error("鎵归噺鍙栨秷杩囨湡璁㈠崟澶辫触",
			logger.Error(err),
			logger.Int("orderCount", len(cancelableOrderIDs)))
		return err
	}
	j.l.Info("鎵归噺鍙栨秷杩囨湡璁㈠崟鎴愬姛", logger.Int("count", len(cancelableOrderIDs)))
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
			j.l.Error("鎵归噺鍙栨秷杩囨湡璁㈠崟澶辫触",
				logger.Error(err),
				logger.Int("batchIndex", i/j.maxBatchSize+1),
				logger.Int("batchSize", len(batch)))
			return err
		}
		j.l.Info("瀹屾垚涓€鎵硅繃鏈熻鍗曞彇娑?,
			logger.Int("batchIndex", i/j.maxBatchSize+1),
			logger.Int("batchSize", len(batch)))
	}
	return nil
}


