package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

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
		j.logger.Error("mark expired coupons failed", logger.Error(err))
		return err
	}

	if affected > 0 {
		j.logger.Info("expired coupons updated", logger.Int64("count", affected))
		if affected > 5000 {
			j.logger.Warn("expired coupon batch is large",
				logger.Int64("count", affected),
				logger.String("suggest", "consider splitting the expiration job into smaller batches"),
			)
		}
	}

	return nil
}
