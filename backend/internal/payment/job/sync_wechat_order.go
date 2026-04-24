package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type SyncPaymentOrderJob struct {
	syncUC *usecase.SyncPaymentOrderUC
	repo   domain.PaymentRepository
	l      logger.LoggerV1
}

func NewSyncPaymentOrderJob(
	syncUC *usecase.SyncPaymentOrderUC,
	repo domain.PaymentRepository,
	l logger.LoggerV1,
) *SyncPaymentOrderJob {
	return &SyncPaymentOrderJob{
		syncUC: syncUC,
		repo:   repo,
		l:      l,
	}
}

func (s *SyncPaymentOrderJob) Name() string {
	return "sync_payment_order_job"
}

func (s *SyncPaymentOrderJob) Run() error {
	const limit = 100
	threshold := time.Now().Add(-25 * time.Minute)
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
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
			err = s.syncUC.SyncOrderInfo(callCtx, pmt.BizTradeNo)
			callCancel()
			if err != nil {
				s.l.Error("sync payment order failed",
					logger.String("trade_no", pmt.BizTradeNo),
					logger.Error(err))
			}
		}
	}
	return nil
}
