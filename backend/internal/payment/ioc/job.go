package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/payment/job"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/robfig/cron/v3"
)

func InitJobs(
	syncPaymentOrderJob *job.SyncPaymentOrderJob,
	paymentOutboxWorkerJob *job.PaymentOutboxWorkerJob,
	l logger.LoggerV1,
) *cron.Cron {
	c := cron.New(cron.WithSeconds())

	_, err := c.AddFunc("0 */2 * * * ?", func() {
		if err := syncPaymentOrderJob.Run(); err != nil {
			l.Error("sync payment order job failed", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	_, err = c.AddFunc("*/10 * * * * ?", func() {
		if err := paymentOutboxWorkerJob.Run(); err != nil {
			l.Error("payment outbox worker job failed", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	l.Info("payment cron jobs initialized")
	return c
}
