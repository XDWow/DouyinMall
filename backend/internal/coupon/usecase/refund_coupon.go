package usecase

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

type RefundCouponUseCase struct {
	couponRepo domain.CouponRepository
}

func NewRefundCouponUseCase(couponRepo domain.CouponRepository) *RefundCouponUseCase {
	return &RefundCouponUseCase{couponRepo: couponRepo}
}

type RefundCouponInput struct {
	OrderID int64
}

func (uc *RefundCouponUseCase) Execute(ctx context.Context, input RefundCouponInput) error {
	// 校验参数
	if input.OrderID <= 0 {
		return errors.New("invalid order_id")
	}

	return uc.couponRepo.UpdateStatusByOrderID(ctx, input.OrderID, domain.UserCouponStatusUsed, domain.UserCouponStatusUnused)
}
