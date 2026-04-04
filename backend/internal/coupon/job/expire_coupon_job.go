package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// ExpireCouponJob 瀹氭湡鎵弿杩囨湡浼樻儬鍒稿苟鏍囪涓哄凡杩囨湡鐘舵€?type ExpireCouponJob struct {
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
		j.logger.Error("鏍囪杩囨湡浼樻儬鍒稿け璐?, logger.Error(err))
		return err
	}

	if affected > 0 {
		j.logger.Info("鏍囪杩囨湡浼樻儬鍒告垚鍔?,
			logger.Int64("count", affected))

		// 濡傛灉鍗曟澶勭悊鏁伴噺寮傚父澶氾紙>5000锛夛紝璇存槑鍙兘绉疮浜嗗ぇ閲忚繃鏈熷埜锛屽彲鑳藉鏁版嵁搴撳帇鍔涜緝澶э紝鎵撲釜鏃ュ織鎻愰啋涓€涓?		if affected > 5000 {
			j.logger.Warn("杩囨湡浼樻儬鍒告暟閲忔湁鐐瑰",
				logger.Int64("count", affected),
				logger.String("suggest", "妫€鏌ュ畾鏃朵换鍔℃槸鍚︽甯歌繍琛?))
		}
	}

	return nil
}


