package job

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"

	"time"
)

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
	offset := 0
	// 也可以做成参数
	const limit = 100
	// 三十分钟之前的订单就认为已经过期了。
	t := time.Now().Add(-time.Minute * 30)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		pmts, err := s.repo.FindExpiredPayment(ctx, offset, limit, t)
		cancel()
		if err != nil {
			// 直接中断，你也可以仔细区别不同错误
			return err
		}
		// 为什么要还去查微信，而不是直接更新为取消呢？因为真实状态只能来自微信，由于回调的延迟、出错，状态不一定是取消
		// 而微信没有批量接口，所以这里也只能单个查询
		for _, pmt := range pmts {
			// 单个重新设置超时
			ctx, cancel = context.WithTimeout(context.Background(), time.Second)
			err = s.syncUC.SyncWechatInfo(ctx, pmt.BizTradeNo)
			if err != nil {
				// 也可以中断，个人倾向处理完毕，本次能处理多少算多少，失败的下次处理
				s.l.Error("同步微信支付信息失败",
					logger.String("trade_no", pmt.BizTradeNo),
					logger.Error(err))
			}
			cancel()
		}
		if len(pmts) <= limit { // 基操：查Limit+1，看这个+1有没有查出来，没有就是没数据了
			return nil
		}
		offset = offset + len(pmts)
	}
}
