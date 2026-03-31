package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/payment/job"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/robfig/cron/v3"
)

// InitJobs 初始化支付服务定时任务。
func InitJobs(
	syncWechatOrderJob *job.SyncWechatOrderJob,
	l logger.LoggerV1,
) *cron.Cron {
	c := cron.New(cron.WithSeconds())

	// 每 2 分钟执行一次，用于支付回调兜底同步。
	_, err := c.AddFunc("0 */2 * * * ?", func() {
		if err := syncWechatOrderJob.Run(); err != nil {
			l.Error("同步微信支付单任务执行失败", logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	l.Info("支付服务定时任务初始化完成")
	return c
}
