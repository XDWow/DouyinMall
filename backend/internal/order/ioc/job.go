package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/order/job"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/robfig/cron/v3"
)

func InitJobs(
	dispatchOrderTimeoutJob *job.DispatchOrderTimeoutJob,
	checkExpiredJob *job.CheckExpiredJob,
	outboxWorkerJob *job.OutboxWorkerJob,
	l logger.LoggerV1,
) *cron.Cron {
	c := cron.New(cron.WithSeconds())

	_, err := c.AddFunc("*/1 * * * * ?", func() {
		if err := dispatchOrderTimeoutJob.Run(); err != nil {
			l.Error("DispatchOrderTimeoutJob鎵ц澶辫触", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	_, err = c.AddFunc("0 */5 * * * ?", func() {
		if err := checkExpiredJob.Run(); err != nil {
			l.Error("CheckExpiredJob鎵ц澶辫触", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	_, err = c.AddFunc("*/30 * * * * ?", func() {
		if err := outboxWorkerJob.Run(); err != nil {
			l.Error("OutboxWorkerJob鎵ц澶辫触", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	l.Info("瀹氭椂浠诲姟鍒濆鍖栧畬鎴?)
	return c
}


