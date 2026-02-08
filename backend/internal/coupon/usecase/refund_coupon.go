package usecase

import (
	"context"

	"DouyinMall/internal/coupon/domain"
)

// ==================== 退还优惠券用例（订单退款） ====================

type RefundCouponUseCase struct {
	couponRepo domain.UserCouponRepository
}

func NewRefundCouponUseCase(couponRepo domain.UserCouponRepository) *RefundCouponUseCase {
	return &RefundCouponUseCase{couponRepo: couponRepo}
}

type RefundCouponInput struct {
	OrderID int64
}

func (uc *RefundCouponUseCase) Execute(ctx context.Context, input RefundCouponInput) error {
	// 直接在数据库层面执行原子操作
	// UPDATE user_coupons SET status=Unused, used_order_id=0, used_at=NULL, valid_end_time=?
	// WHERE order_id=? AND status=Used
	// 返回受影响行数，0表示没有需要退还的优惠券（幂等成功）
	_, err := uc.couponRepo.RefundByOrderID(ctx, input.OrderID)
	if err != nil {
		return err
	}
	// 受影响行数为0表示没有需要退还的优惠券，这是幂等的，直接返回成功
	return nil
}
