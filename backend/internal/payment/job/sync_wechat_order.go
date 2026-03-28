package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// SyncWechatOrderJob 用于在支付回调缺失时做最终兜底同步。
type SyncWechatOrderJob struct {
	syncUC *usecase.SyncWechatOrderUC
	repo   domain.PaymentRepository
	l      logger.LoggerV1
}

func NewSyncWechatOrderJob(
	syncUC *usecase.SyncWechatOrderUC,
	repo domain.PaymentRepository,
	l logger.LoggerV1,
) *SyncWechatOrderJob {
	return &SyncWechatOrderJob{
		syncUC: syncUC,
		repo:   repo,
		l:      l,
	}
}

func (s *SyncWechatOrderJob) Name() string {
	return "sync_wechat_order_job"
}

func (s *SyncWechatOrderJob) Run() error {
	const limit = 100
	threshold := time.Now().Add(-25 * time.Minute)
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) { // 只跑 30s，避免一个 crobJob 持续太久
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pmts, err := s.repo.FindExpiredPayment(ctx, limit, threshold)
		cancel()
		if err != nil {
			return err
		}
		if len(pmts) == 0 {
			return nil
		}

		for _, pmt := range pmts {
			if time.Now().After(deadline) {
				return nil
			}

			callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = s.syncUC.SyncWechatInfo(callCtx, pmt.BizTradeNo)
			callCancel()
			if err != nil {
				s.l.Error("同步微信支付单失败",
					logger.String("trade_no", pmt.BizTradeNo),
					logger.Error(err))
			}
		}
	}
	return nil
}
