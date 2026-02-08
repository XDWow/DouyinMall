package usecase

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

type ListUserCouponsUseCase struct {
	couponRepo domain.CouponRepository
}

func NewListUserCouponsUseCase(couponRepo domain.CouponRepository) *ListUserCouponsUseCase {
	return &ListUserCouponsUseCase{couponRepo: couponRepo}
}

type ListUserCouponsInput struct {
	UserID   int64
	Status   domain.CouponStatus
	Page     int
	PageSize int
}

type ListUserCouponsOutput struct {
	Coupons []domain.Coupon
	Total   int32
}

func (uc *ListUserCouponsUseCase) Execute(ctx context.Context, input ListUserCouponsInput) (*ListUserCouponsOutput, error) {
	coupons, total, err := uc.couponRepo.ListByUserID(ctx, input.UserID, input.Status, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	return &ListUserCouponsOutput{
		Coupons: coupons,
		Total:   total,
	}, nil
}

