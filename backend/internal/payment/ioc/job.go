package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/order/job"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/robfig/cron/v3"
)

// InitJobs 初始化所有定时任务，使用cron调度
func InitJobs(
	checkExpiredJob *job.CheckExpiredJob,
	outboxWorkerJob *job.OutboxWorkerJob,
	l logger.LoggerV1,
) *cron.Cron {
	c := cron.New(cron.WithSeconds())

	// 每分钟检查一次过期订单
	_, err := c.AddFunc("0 * * * * ?", func() {
		if err := checkExpiredJob.Run(); err != nil {
			l.Error("CheckExpiredJob执行失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	// 每30秒检查一次待发送的outbox事件
	_, err = c.AddFunc("*/30 * * * * ?", func() {
		if err := outboxWorkerJob.Run(); err != nil {
			l.Error("OutboxWorkerJob执行失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	l.Info("定时任务初始化完成")
	return c
}
