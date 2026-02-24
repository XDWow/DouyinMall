package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// ExpireCouponJob 定期扫描过期优惠券并标记为已过期状态
type ExpireCouponJob struct {
	couponRepo domain.CouponRepository
	logger     logger.LoggerV1
}

func NewExpireCouponJob(
	couponRepo domain.CouponRepository,
	l logger.LoggerV1,
) *ExpireCouponJob {
	return &ExpireCouponJob{
		couponRepo: couponRepo,
		logger:     l,
	}
}

func (j *ExpireCouponJob) Name() string {
	return "ExpireCouponJob"
}

func (j *ExpireCouponJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	affected, err := j.couponRepo.MarkExpiredCoupons(ctx)
	if err != nil {
		j.logger.Error("标记过期优惠券失败", logger.Error(err))
		return err
	}

	if affected > 0 {
		j.logger.Info("标记过期优惠券成功",
			logger.Int64("count", affected))

		// 如果单次处理数量异常多（>5000），说明可能积累了大量过期券，可能对数据库压力较大，打个日志提醒一下
		if affected > 5000 {
			j.logger.Warn("过期优惠券数量有点多",
				logger.Int64("count", affected),
				logger.String("suggest", "检查定时任务是否正常运行"))
		}
	}

	return nil
}
