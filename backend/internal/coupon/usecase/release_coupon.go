package usecase

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

type ReleaseCouponUseCase struct {
	couponRepo domain.CouponRepository
}

func NewReleaseCouponUseCase(couponRepo domain.CouponRepository) *ReleaseCouponUseCase {
	return &ReleaseCouponUseCase{couponRepo: couponRepo}
}

type ReleaseCouponInput struct {
	OrderID int64
}

func (uc *ReleaseCouponUseCase) Execute(ctx context.Context, input ReleaseCouponInput) error {
	// 鏍￠獙鍙傛暟
	if input.OrderID <= 0 {
		return errors.New("invalid order_id")
	}

	return uc.couponRepo.UpdateStatusByOrderID(ctx, input.OrderID, domain.UserCouponStatusLocked, domain.UserCouponStatusUnused)
}


