package usecase

import (
	"context"

	"DouyinMall/internal/coupon/domain"
)

// ==================== 释放优惠券用例（订单取消） ====================

type ReleaseCouponUseCase struct {
	couponRepo domain.UserCouponRepository
}

func NewReleaseCouponUseCase(couponRepo domain.UserCouponRepository) *ReleaseCouponUseCase {
	return &ReleaseCouponUseCase{couponRepo: couponRepo}
}

type ReleaseCouponInput struct {
	OrderID int64
}

func (uc *ReleaseCouponUseCase) Execute(ctx context.Context, input ReleaseCouponInput) error {
	// 直接在数据库层面执行原子操作
	// UPDATE user_coupons SET status=Unused, locked_order_id=0 
	// WHERE order_id=? AND status=Locked
	// 返回受影响行数，0表示没有需要释放的优惠券（幂等成功）
	_, err := uc.couponRepo.ReleaseByOrderID(ctx, input.OrderID)
	if err != nil {
		return err
	}
	// 受影响行数为0表示没有需要释放的优惠券，这是幂等的，直接返回成功
	return nil
}
