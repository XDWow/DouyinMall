package usecase

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

type EvaluateOrderCouponsInput struct {
	UserID int64
	Items  []domain.OrderItem // 璁㈠崟鍟嗗搧鍒楄〃锛圕heckout 浼犲叆锛屽彧鍚晢鍝佸師浠凤級
}

type EvaluateOrderCouponsOutput struct {
	Coupons []EvaluatedCoupon
}

// 缁撶畻椤典紭鎯犲埜璇勪及缁撴灉
type EvaluatedCoupon struct {
	Coupon *domain.Coupon
	Usable bool
	Reason string // 涓嶅彲鐢ㄥ師鍥?
}

// 鍦ㄧ粨绠楅〉涓壘鍑烘墍鏈夋湰璁㈠崟鍙敤鐨勪紭鎯犲埜锛岀粰鐢ㄦ埛閫夋嫨
type EvaluateOrderCouponsUseCase struct {
	couponRepo domain.CouponRepository
}

func NewEvaluateOrderCouponsUseCase(
	couponRepo domain.CouponRepository,
) *EvaluateOrderCouponsUseCase {
	return &EvaluateOrderCouponsUseCase{
		couponRepo: couponRepo,
	}
}

func (uc *EvaluateOrderCouponsUseCase) Execute(
	ctx context.Context,
	input EvaluateOrderCouponsInput,
) (*EvaluateOrderCouponsOutput, error) {
	if len(input.Items) == 0 || input.UserID < 0 {
		return nil, fmt.Errorf("invalid input")
	}

	// 鏌ヨ鐢ㄦ埛灞傞潰鍙敤鐨勪紭鎯犲埜锛堟湭浣跨敤 && 鏈繃鏈燂級
	coupons, err := uc.couponRepo.ListAvailableByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	results := make([]EvaluatedCoupon, 0, len(coupons))
	for _, c := range coupons {
		// 璁㈠崟灞傞潰妫€鏌ワ細閫傜敤鑼冨洿 + 闂ㄦ
		usable, reason := uc.evaluateCoupon(c, input)
		results = append(results, EvaluatedCoupon{
			Coupon: c,
			Usable: usable,
			Reason: reason,
		})
	}

	return &EvaluateOrderCouponsOutput{Coupons: results}, nil
}

// 璁㈠崟缁村害鐨勯€傜敤鎬ф鏌ワ紝鏈潵閫昏緫鍦ㄨ繖涓柟娉曚腑锛屽悗闈紭鍖栦簡
// 鎶婇€昏緫鏀?domain 灞傦紝鍥犱负"鍒ゆ柇鍒歌兘鍚︾敤浜庤鍗?鏄紭鎯犲埜妯℃澘鐨勫浐鏈夎亴璐?
// 绗﹀悎 DDD 棰嗗煙椹卞姩璁捐锛屼笟鍔¤鍒欑敱棰嗗煙瀵硅薄璐熻矗
func (uc *EvaluateOrderCouponsUseCase) evaluateCoupon(
	c *domain.Coupon,
	input EvaluateOrderCouponsInput,
) (bool, string) {
	tpl := c.Template
	if tpl == nil {
		return false, "鍒告ā鏉夸笉瀛樺湪"
	}

	// 妫€鏌ラ€傜敤鎬э紙璁＄畻閫傜敤閲戦 + 妫€鏌ラ棬妲涳級
	return tpl.IsApplicableToOrder(input.Items)
}


