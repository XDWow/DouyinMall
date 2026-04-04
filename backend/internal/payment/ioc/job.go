package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/payment/job"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/robfig/cron/v3"
)

// InitJobs 鍒濆鍖栨敮浠樻湇鍔″畾鏃朵换鍔°€?
func InitJobs(
	syncWechatOrderJob *job.SyncWechatOrderJob,
	l logger.LoggerV1,
) *cron.Cron {
	c := cron.New(cron.WithSeconds())

	// 姣?2 鍒嗛挓鎵ц涓€娆★紝鐢ㄤ簬鏀粯鍥炶皟鍏滃簳鍚屾銆?
	_, err := c.AddFunc("0 */2 * * * ?", func() {
		if err := syncWechatOrderJob.Run(); err != nil {
			l.Error("鍚屾寰俊鏀粯鍗曚换鍔℃墽琛屽け璐?, logger.Error(err))
		}
	})
	if err != nil {
		panic(err)
	}

	l.Info("鏀粯鏈嶅姟瀹氭椂浠诲姟鍒濆鍖栧畬鎴?)
	return c
}


