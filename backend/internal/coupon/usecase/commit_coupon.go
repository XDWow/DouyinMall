package usecase

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

type CommitCouponUseCase struct {
	couponRepo domain.CouponRepository
}

func NewCommitCouponUseCase(couponRepo domain.CouponRepository) *CommitCouponUseCase {
	return &CommitCouponUseCase{couponRepo: couponRepo}
}

type CommitCouponInput struct {
	OrderID int64
}

func (uc *CommitCouponUseCase) Execute(ctx context.Context, input CommitCouponInput) error {
	if input.OrderID <= 0 {
		return errors.New("invalid order_id")
	}
	return uc.couponRepo.UpdateStatusByOrderID(ctx, input.OrderID, domain.UserCouponStatusLocked, domain.UserCouponStatusUsed)
}
