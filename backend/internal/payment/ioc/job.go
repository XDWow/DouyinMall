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
			l.Error("同步支付订单任务失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	_, err = c.AddFunc("*/10 * * * * ?", func() {
		if err := paymentOutboxWorkerJob.Run(); err != nil {
			l.Error("支付 outbox 投递任务失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	l.Info("支付定时任务已初始化")
	return c
}
