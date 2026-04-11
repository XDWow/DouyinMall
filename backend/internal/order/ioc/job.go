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
			l.Error("DispatchOrderTimeoutJob 执行失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	_, err = c.AddFunc("0 */5 * * * ?", func() {
		if err := checkExpiredJob.Run(); err != nil {
			l.Error("CheckExpiredJob 执行失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	_, err = c.AddFunc("*/30 * * * * ?", func() {
		if err := outboxWorkerJob.Run(); err != nil {
			l.Error("OutboxWorkerJob 执行失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	l.Info("定时任务初始化完成")
	return c
}


